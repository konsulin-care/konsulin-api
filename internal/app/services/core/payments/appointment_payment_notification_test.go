package payments

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/fhir_dto"
	"konsulin-service/internal/pkg/fhir_http_client"

	"go.uber.org/zap"
)

// mockSlotFhirClient mocks contracts.SlotFhirClient for notification tests.
type mockSlotFhirClient struct {
	slot            *fhir_dto.Slot
	err             error
	slotsBySchedule func(scheduleID string, params contracts.SlotSearchParams) ([]fhir_dto.Slot, error)
}

func (m *mockSlotFhirClient) FindSlotByID(_ context.Context, _ string) (*fhir_dto.Slot, error) {
	return m.slot, m.err
}
func (*mockSlotFhirClient) FindSlotByScheduleID(_ context.Context, _ string) ([]fhir_dto.Slot, error) {
	return nil, nil
}
func (*mockSlotFhirClient) FindSlotByScheduleAndTimeRange(_ context.Context, _ string, _, _ time.Time) ([]fhir_dto.Slot, error) {
	return nil, nil
}
func (*mockSlotFhirClient) FindSlotByScheduleIDAndStatus(_ context.Context, _, _ string) ([]fhir_dto.Slot, error) {
	return nil, nil
}
func (*mockSlotFhirClient) CreateSlot(_ context.Context, _ *fhir_dto.Slot) (*fhir_dto.Slot, error) {
	return nil, nil
}
func (*mockSlotFhirClient) UpdateSlot(_ context.Context, _ string, _ *fhir_dto.Slot) (*fhir_dto.Slot, error) {
	return nil, nil
}
func (m *mockSlotFhirClient) FindSlotsByScheduleWithQuery(_ context.Context, scheduleID string, params contracts.SlotSearchParams) ([]fhir_dto.Slot, error) {
	if m.slotsBySchedule != nil {
		return m.slotsBySchedule(scheduleID, params)
	}
	return nil, nil
}
func (*mockSlotFhirClient) PostTransactionBundle(_ context.Context, _ map[string]interface{}) (*fhir_dto.FHIRBundle, error) {
	return nil, nil
}

// mockSlotUsecase mocks contracts.SlotUsecaseIface for notification tests.
type mockSlotUsecase struct {
	lockErr          error
	lockPractitioner string
	lockStart        time.Time
	lockEnd          time.Time
}

func (*mockSlotUsecase) HandleSetUnavailabilityForMultiplePractitionerRoles(_ context.Context, _ contracts.SetUnavailabilityForMultiplePractitionerRolesInput) (*contracts.SetUnavailableOutcome, error) {
	return nil, nil
}
func (m *mockSlotUsecase) AcquireLocksForPractitionerDay(_ context.Context, practitionerID string, start, end time.Time, _ time.Duration) (func(context.Context), error) {
	m.lockPractitioner = practitionerID
	m.lockStart = start
	m.lockEnd = end
	if m.lockErr != nil {
		return func(_ context.Context) {}, m.lockErr
	}
	return func(_ context.Context) {}, nil
}

// mockBundleFhirClient mocks bundleSvc.BundleFhirClient, capturing every posted bundle.
type mockBundleFhirClient struct {
	bundles []map[string]any
	err     error
}

func (m *mockBundleFhirClient) PostTransactionBundle(_ context.Context, bundle map[string]any) (*fhir_dto.FHIRBundle, error) {
	m.bundles = append(m.bundles, bundle)
	return &fhir_dto.FHIRBundle{}, m.err
}

// mockPatientFhirClient mocks contracts.PatientFhirClient for payment tests.
type mockPatientFhirClient struct {
	patient *fhir_dto.Patient
	err     error
}

func (m *mockPatientFhirClient) FindPatientByID(_ context.Context, _ string) (*fhir_dto.Patient, error) {
	return m.patient, m.err
}
func (*mockPatientFhirClient) CreatePatient(_ context.Context, _ *fhir_dto.Patient) (*fhir_dto.Patient, error) {
	return nil, nil
}
func (*mockPatientFhirClient) UpdatePatient(_ context.Context, _ *fhir_dto.Patient) (*fhir_dto.Patient, error) {
	return nil, nil
}
func (*mockPatientFhirClient) PatchPatient(_ context.Context, _ *fhir_dto.Patient) (*fhir_dto.Patient, error) {
	return nil, nil
}
func (*mockPatientFhirClient) FindPatientByIdentifier(_ context.Context, _ string) ([]fhir_dto.Patient, error) {
	return nil, nil
}
func (*mockPatientFhirClient) FindPatientByEmail(_ context.Context, _ string) ([]fhir_dto.Patient, error) {
	return nil, nil
}
func (*mockPatientFhirClient) FindPatientByPhone(_ context.Context, _ string) ([]fhir_dto.Patient, error) {
	return nil, nil
}

// mockAppointmentResourceClient stubs the raw FHIR HTTP operations used to fetch
// Appointment and HealthcareService resources during payment callbacks.
type mockAppointmentResourceClient struct {
	getResponses map[string][]byte
	gets         []string
}

func (m *mockAppointmentResourceClient) Do(_ context.Context, method, url string, _ io.Reader) ([]byte, error) {
	if method == http.MethodGet {
		m.gets = append(m.gets, url)
		if body := m.getResponses[url]; body != nil {
			return body, nil
		}
		return nil, &fhir_http_client.FHIRHTTPError{StatusCode: http.StatusNotFound, Err: errors.New("resource not found")}
	}
	return nil, fmt.Errorf("unexpected method %s for %s", method, url)
}

// findBundleEntry returns the first entry in a transaction bundle whose request
// URL matches the given resource path (e.g. "Appointment/appt-999").
func findBundleEntry(bundle map[string]any, url string) map[string]any {
	entries, ok := bundle[constvars.FhirFieldEntry].([]map[string]any)
	if !ok {
		return nil
	}
	for _, entry := range entries {
		req, ok := entry[constvars.FhirFieldRequest].(map[string]any)
		if !ok {
			continue
		}
		if entryURL, _ := req[constvars.FhirFieldURL].(string); entryURL == url {
			return entry
		}
	}
	return nil
}

// assertBundleDeletes verifies the bundle contains exactly one DELETE entry per
// given URL, in order.
func assertBundleDeletes(t *testing.T, bundle map[string]any, urls ...string) {
	t.Helper()
	entries, ok := bundle[constvars.FhirFieldEntry].([]map[string]any)
	if !ok {
		t.Fatal("bundle missing entry list")
	}
	if len(entries) != len(urls) {
		t.Fatalf("expected %d entries, got %d", len(urls), len(entries))
	}
	for i, want := range urls {
		entry := entries[i]
		req, ok := entry[constvars.FhirFieldRequest].(map[string]any)
		if !ok {
			t.Fatalf("entry %d missing request", i)
		}
		method, _ := req[constvars.FhirFieldMethod].(string)
		entryURL, _ := req[constvars.FhirFieldURL].(string)
		if method != "DELETE" {
			t.Errorf("entry %d: expected DELETE method, got %q", i, method)
		}
		if entryURL != want {
			t.Errorf("entry %d: expected url %q, got %q", i, want, entryURL)
		}
	}
}

func TestHandleAppointmentPaymentNotification_ExpiredSlotNotFound(t *testing.T) {
	t.Run("EXPIRED callback returns nil when slot is already deleted", func(t *testing.T) {
		uc := &paymentUsecase{
			SlotFhirClient: &mockSlotFhirClient{
				slot: nil,
				err:  errors.New("slot not found"),
			},
			SlotUsecase: &mockSlotUsecase{},
			Log:         zap.NewNop(),
		}

		// externalID format: appointment:{slotID}:{practitionerRoleID}:{patientID}:{invoiceID}:{healthcareServiceID}:{appointmentID}
		externalID := "appointment:deleted-slot-123:pr-456:pat-789:inv-012:hs-999:appt-000"
		err := uc.handleAppointmentPaymentNotification(
			context.Background(),
			externalID,
			requests.XenditInvoiceStatusExpired,
		)

		// Should not error because an already-deleted slot for EXPIRED is idempotent
		if err != nil {
			t.Errorf("expected nil for EXPIRED callback on missing slot, got: %v", err)
		}
	})

	t.Run("PAID callback still returns error when slot is not found", func(t *testing.T) {
		uc := &paymentUsecase{
			SlotFhirClient: &mockSlotFhirClient{
				slot: nil,
				err:  errors.New("slot not found"),
			},
			SlotUsecase: &mockSlotUsecase{},
			Log:         zap.NewNop(),
		}

		externalID := "appointment:missing-slot-456:pr-456:pat-789:inv-012:hs-999:appt-000"
		err := uc.handleAppointmentPaymentNotification(
			context.Background(),
			externalID,
			requests.XenditInvoiceStatusPaid,
		)

		// Should still return error for non-EXPIRED status
		if err == nil {
			t.Error("expected error for PAID callback on missing slot, got nil")
		}
	})
}

func TestHandleAppointmentPaymentNotification_ExpiredDeletesAllResources(t *testing.T) {
	t.Run("EXPIRED callback deletes slot, invoice, and appointment atomically", func(t *testing.T) {
		slotID := "slot-123"
		bundleClient := &mockBundleFhirClient{}
		uc := &paymentUsecase{
			PractitionerRoleFhirClient: &mockPractitionerRoleFhirClient{
				role: &fhir_dto.PractitionerRole{
					Practitioner: fhir_dto.Reference{Reference: "Practitioner/prac-9"},
				},
			},
			SlotFhirClient: &mockSlotFhirClient{
				slot: &fhir_dto.Slot{
					ID:     slotID,
					Status: fhir_dto.SlotStatusBusyTentative,
					Start:  time.Date(2026, 7, 4, 9, 0, 0, 0, time.UTC),
					End:    time.Date(2026, 7, 4, 10, 0, 0, 0, time.UTC),
				},
			},
			SlotUsecase:     &mockSlotUsecase{},
			BundleFhirClient: bundleClient,
			Log:             zap.NewNop(),
		}

		externalID := fmt.Sprintf("appointment:%s:pr-456:pat-789:inv-012:hs-999:appt-000", slotID)
		err := uc.handleAppointmentPaymentNotification(
			context.Background(),
			externalID,
			requests.XenditInvoiceStatusExpired,
		)

		if err != nil {
			t.Errorf("expected nil for EXPIRED callback, got: %v", err)
		}
		if len(bundleClient.bundles) != 1 {
			t.Fatalf("expected 1 posted bundle, got %d", len(bundleClient.bundles))
		}
		assertBundleDeletes(t, bundleClient.bundles[0],
			"Slot/"+slotID,
			"Invoice/inv-012",
			"Appointment/appt-000",
		)
	})
}

func TestHandleAppointmentPaymentPaid_TransitionsAppointmentToBooked(t *testing.T) {
	appointmentID := "appt-999"
	fields := appointmentExternalIDFields{
		SlotID:              "slot-123",
		PractitionerRoleID:  "pr-456",
		PatientID:           "pat-789",
		InvoiceID:           "inv-012",
		HealthcareServiceID: "hs-999",
		AppointmentID:       appointmentID,
	}

	slotStart := time.Date(2026, 7, 4, 9, 0, 0, 0, time.UTC)
	slotEnd := time.Date(2026, 7, 4, 10, 0, 0, 0, time.UTC)

	slot := &fhir_dto.Slot{
		ID:       "slot-123",
		Status:   fhir_dto.SlotStatusBusyTentative,
		Start:    slotStart,
		End:      slotEnd,
		Schedule: fhir_dto.Reference{Reference: "Schedule/sched-1"},
	}

	pendingAppointment := &fhir_dto.Appointment{
		ResourceType: constvars.ResourceAppointment,
		ID:           appointmentID,
		Status:       constvars.FhirAppointmentStatusPending,
		Start:        slotStart,
		End:          slotEnd,
		Slot: []fhir_dto.Reference{
			{Reference: "Slot/slot-123"},
		},
		Participant: []fhir_dto.AppointmentParticipant{
			{Actor: fhir_dto.Reference{Reference: "Patient/pat-789"}, Status: constvars.FhirParticipantStatusAccepted},
			{Actor: fhir_dto.Reference{Reference: "PractitionerRole/pr-456"}, Status: constvars.FhirParticipantStatusAccepted},
		},
	}

	appointmentJSON, _ := json.Marshal(pendingAppointment)
	hsJSON, _ := json.Marshal(&fhir_dto.HealthcareService{
		ResourceType: constvars.ResourceHealthcareService,
		ID:           "hs-999",
	})

	fhirClient := &mockAppointmentResourceClient{
		getResponses: map[string][]byte{
			"http://fhir.test/Appointment/appt-999":     appointmentJSON,
			"http://fhir.test/HealthcareService/hs-999": hsJSON,
		},
	}

	bundleClient := &mockBundleFhirClient{}

	uc := &paymentUsecase{
		InternalConfig: &config.InternalConfig{
			FHIR: config.AppFHIR{BaseUrl: "http://fhir.test/"},
			App:  config.App{BaseUrl: "http://app.test"},
		},
		FHIRClient: fhirClient,
		PatientFhirClient: &mockPatientFhirClient{
			patient: &fhir_dto.Patient{ID: "pat-789"},
		},
		PractitionerFhirClient: &mockPractitionerFhirClient{
			practitioner: &fhir_dto.Practitioner{ID: "prac-001"},
		},
		PractitionerRoleFhirClient: &mockPractitionerRoleFhirClient{
			role: &fhir_dto.PractitionerRole{
				ID:           "pr-456",
				Practitioner: fhir_dto.Reference{Reference: "Practitioner/prac-001"},
			},
		},
		InvoiceFhirClient: &mockInvoiceFhirClient{
			invoices: []fhir_dto.Invoice{
				{
					ID:       "inv-012",
					TotalNet: &fhir_dto.Money{Value: 150000, Currency: "IDR"},
				},
			},
		},
		SlotFhirClient:     &mockSlotFhirClient{slot: slot},
		ScheduleFhirClient: &mockScheduleFhirClient{},
		BundleFhirClient:   bundleClient,
		SlotUsecase:        &mockSlotUsecase{},
		Log:                zap.NewNop(),
	}

	err := uc.handleAppointmentPaymentPaid(context.Background(), fields, slot)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(bundleClient.bundles) != 1 {
		t.Fatalf("expected 1 posted bundle, got %d", len(bundleClient.bundles))
	}
	bundle := bundleClient.bundles[0]

	// The appointment must be fetched and included as a PUT with status booked
	appointmentEntry := findBundleEntry(bundle, constvars.ResourceAppointment+"/"+appointmentID)
	if appointmentEntry == nil {
		t.Fatal("expected appointment PUT entry in bundle")
	}
	apptResource, ok := appointmentEntry[constvars.FhirFieldResource].(*fhir_dto.Appointment)
	if !ok {
		t.Fatal("appointment entry missing resource")
	}
	if apptResource.Status != constvars.FhirAppointmentStatusBooked {
		t.Errorf("expected appointment status %q in bundle, got %q", constvars.FhirAppointmentStatusBooked, apptResource.Status)
	}

	// The slot must be updated to busy-unavailable in the same bundle
	slotEntry := findBundleEntry(bundle, constvars.ResourceSlot+"/slot-123")
	if slotEntry == nil {
		t.Fatal("expected slot PUT entry in bundle")
	}
	slotResource, ok := slotEntry[constvars.FhirFieldResource].(*fhir_dto.Slot)
	if !ok {
		t.Fatal("slot entry missing resource")
	}
	if slotResource.Status != fhir_dto.SlotStatusBusyUnavailable {
		t.Errorf("expected slot status %q in bundle, got %q", fhir_dto.SlotStatusBusyUnavailable, slotResource.Status)
	}

	// The appointment must have been fetched via the FHIR client
	if !containsString(fhirClient.gets, "http://fhir.test/Appointment/appt-999") {
		t.Errorf("expected appointment fetch, got GETs: %v", fhirClient.gets)
	}
}

func containsString(list []string, target string) bool {
	for _, s := range list {
		if s == target {
			return true
		}
	}
	return false
}
