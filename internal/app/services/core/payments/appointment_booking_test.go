package payments

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"testing"
	"time"

	"konsulin-service/internal/app/config"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/fhir_dto"

	"go.uber.org/zap"
)

// recordingSlotFhirClient records every UpdateSlot call so tests can assert the
// slot status transitions emitted by handleAppointmentBooking.
type recordingSlotFhirClient struct {
	mockSlotFhirClient
	updatedSlots []*fhir_dto.Slot
}

func (m *recordingSlotFhirClient) UpdateSlot(_ context.Context, _ string, slot *fhir_dto.Slot) (*fhir_dto.Slot, error) {
	// snapshot: handleAppointmentBooking mutates the same pointer on rollback,
	// so storing the pointer would let the second update rewrite the first record.
	cp := *slot
	m.updatedSlots = append(m.updatedSlots, &cp)
	return slot, nil
}

// recordingAppointmentResourceClient records every PUT body sent through the
// raw FHIR client so tests can assert the appointment status transitions
// emitted by handleAppointmentBooking.
type recordingAppointmentResourceClient struct {
	mockAppointmentResourceClient
	putBodies []map[string]any
	putErr    error
}

func (m *recordingAppointmentResourceClient) Do(ctx context.Context, method, url string, body io.Reader) ([]byte, error) {
	if method == http.MethodPut {
		raw, _ := io.ReadAll(body)
		var payload map[string]any
		_ = json.Unmarshal(raw, &payload)
		m.putBodies = append(m.putBodies, payload)
		return raw, m.putErr
	}
	return m.mockAppointmentResourceClient.Do(ctx, method, url, body)
}

// bookingTestHarness builds a paymentUsecase wired to recording fakes plus the
// given invoice creator, which the tests use to simulate the Xendit call.
func bookingTestHarness(t *testing.T, invoiceCreator func(context.Context, *requests.AppointmentPaymentRequest, *preconditionData) (string, error)) (*paymentUsecase, *recordingSlotFhirClient, *recordingAppointmentResourceClient) {
	t.Helper()
	uc := &paymentUsecase{
		InternalConfig: &config.InternalConfig{
			FHIR: config.AppFHIR{BaseUrl: "http://fhir.test/"},
			App:  config.App{PaymentExpiredTimeInMinutes: 60},
		},
		SlotFhirClient: &recordingSlotFhirClient{},
		FHIRClient:     &recordingAppointmentResourceClient{},
		Log:            zap.NewNop(),
	}
	uc.CreateAppointmentInvoice = invoiceCreator
	return uc, uc.SlotFhirClient.(*recordingSlotFhirClient), uc.FHIRClient.(*recordingAppointmentResourceClient)
}

func TestHandleAppointmentBooking_ReservesSlotAndMovesAppointmentToPending(t *testing.T) {
	slot := &fhir_dto.Slot{
		ID:     "slot-012",
		Status: fhir_dto.SlotStatusFree,
		Start:  time.Date(2026, 7, 4, 9, 0, 0, 0, time.UTC),
		End:    time.Date(2026, 7, 4, 10, 0, 0, 0, time.UTC),
	}
	appointment := &fhir_dto.Appointment{
		ID:     "appt-000",
		Status: constvars.FhirAppointmentStatusProposed,
	}
	req := validProposedAppointmentRequest()

	invoiceCalls := 0
	uc, slotClient, recorder := bookingTestHarness(t, func(_ context.Context, _ *requests.AppointmentPaymentRequest, _ *preconditionData) (string, error) {
		invoiceCalls++
		return "https://checkout.xendit.co/inv_123", nil
	})

	resp, err := uc.handleAppointmentBooking(
		context.Background(),
		req,
		&preconditionData{},
		appointment,
		slot,
		"slot-012",
		"test-request-id",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if invoiceCalls != 1 {
		t.Errorf("expected 1 invoice creation call, got %d", invoiceCalls)
	}
	if resp.PaymentURL != "https://checkout.xendit.co/inv_123" {
		t.Errorf("expected payment URL, got %q", resp.PaymentURL)
	}
	if resp.AppointmentID != "Appointment/appt-000" {
		t.Errorf("expected appointment %q, got %q", "Appointment/appt-000", resp.AppointmentID)
	}
	if resp.SlotID != req.SlotID {
		t.Errorf("expected slot %q, got %q", req.SlotID, resp.SlotID)
	}

	// Slot must be reserved as busy-tentative before the invoice is created
	if len(slotClient.updatedSlots) != 1 {
		t.Fatalf("expected 1 slot update, got %d", len(slotClient.updatedSlots))
	}
	if status := slotClient.updatedSlots[0].Status; status != fhir_dto.SlotStatusBusyTentative {
		t.Errorf("expected slot status %q, got %q", fhir_dto.SlotStatusBusyTentative, status)
	}

	// Appointment must be moved to pending via a full PUT before the invoice is created
	if len(recorder.putBodies) != 1 {
		t.Fatalf("expected 1 appointment PUT, got %d", len(recorder.putBodies))
	}
	if status := recorder.putBodies[0]["status"]; status != constvars.FhirAppointmentStatusPending {
		t.Errorf("expected appointment status %q, got %v", constvars.FhirAppointmentStatusPending, status)
	}
	if id := recorder.putBodies[0]["id"]; id != "appt-000" {
		t.Errorf("expected appointment id %q, got %v", "appt-000", id)
	}
}

func TestHandleAppointmentBooking_RollsBackWhenInvoiceCreationFails(t *testing.T) {
	slot := &fhir_dto.Slot{
		ID:     "slot-012",
		Status: fhir_dto.SlotStatusFree,
		Start:  time.Date(2026, 7, 4, 9, 0, 0, 0, time.UTC),
		End:    time.Date(2026, 7, 4, 10, 0, 0, 0, time.UTC),
	}
	appointment := &fhir_dto.Appointment{
		ID:     "appt-000",
		Status: constvars.FhirAppointmentStatusProposed,
	}
	req := validProposedAppointmentRequest()

	uc, slotClient, recorder := bookingTestHarness(t, func(_ context.Context, _ *requests.AppointmentPaymentRequest, _ *preconditionData) (string, error) {
		return "", errors.New("xendit unavailable")
	})

	_, err := uc.handleAppointmentBooking(
		context.Background(),
		req,
		&preconditionData{},
		appointment,
		slot,
		"slot-012",
		"test-request-id",
	)
	if err == nil {
		t.Fatal("expected error when invoice creation fails, got nil")
	}

	// Slot: reserved busy-tentative, then rolled back to free
	if len(slotClient.updatedSlots) != 2 {
		t.Fatalf("expected 2 slot updates (reserve + rollback), got %d", len(slotClient.updatedSlots))
	}
	if status := slotClient.updatedSlots[0].Status; status != fhir_dto.SlotStatusBusyTentative {
		t.Errorf("expected first slot status %q, got %q", fhir_dto.SlotStatusBusyTentative, status)
	}
	if status := slotClient.updatedSlots[1].Status; status != fhir_dto.SlotStatusFree {
		t.Errorf("expected rolled-back slot status %q, got %q", fhir_dto.SlotStatusFree, status)
	}

	// Appointment: moved to pending, then rolled back to proposed
	if len(recorder.putBodies) != 2 {
		t.Fatalf("expected 2 appointment PUTs (pending + rollback), got %d", len(recorder.putBodies))
	}
	if status := recorder.putBodies[0]["status"]; status != constvars.FhirAppointmentStatusPending {
		t.Errorf("expected first appointment status %q, got %v", constvars.FhirAppointmentStatusPending, status)
	}
	if status := recorder.putBodies[1]["status"]; status != constvars.FhirAppointmentStatusProposed {
		t.Errorf("expected rolled-back appointment status %q, got %v", constvars.FhirAppointmentStatusProposed, status)
	}
}
