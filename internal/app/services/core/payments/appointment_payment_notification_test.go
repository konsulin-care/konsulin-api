package payments

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/fhir_dto"

	"go.uber.org/zap"
)

// mockSlotFhirClient mocks contracts.SlotFhirClient for notification tests.
type mockSlotFhirClient struct {
	slot *fhir_dto.Slot
	err  error
}

func (m *mockSlotFhirClient) FindSlotByID(_ context.Context, slotID string) (*fhir_dto.Slot, error) {
	return m.slot, m.err
}
func (m *mockSlotFhirClient) FindSlotByScheduleID(_ context.Context, _ string) ([]fhir_dto.Slot, error) {
	return nil, nil
}
func (m *mockSlotFhirClient) FindSlotByScheduleAndTimeRange(_ context.Context, _ string, _, _ time.Time) ([]fhir_dto.Slot, error) {
	return nil, nil
}
func (m *mockSlotFhirClient) FindSlotByScheduleIDAndStatus(_ context.Context, _, _ string) ([]fhir_dto.Slot, error) {
	return nil, nil
}
func (m *mockSlotFhirClient) CreateSlot(_ context.Context, _ *fhir_dto.Slot) (*fhir_dto.Slot, error) {
	return nil, nil
}
func (m *mockSlotFhirClient) UpdateSlot(_ context.Context, _ string, _ *fhir_dto.Slot) (*fhir_dto.Slot, error) {
	return nil, nil
}
func (m *mockSlotFhirClient) FindSlotsByScheduleWithQuery(_ context.Context, _ string, _ contracts.SlotSearchParams) ([]fhir_dto.Slot, error) {
	return nil, nil
}
func (m *mockSlotFhirClient) PostTransactionBundle(_ context.Context, _ map[string]interface{}) (*fhir_dto.FHIRBundle, error) {
	return nil, nil
}

// mockSlotUsecase mocks contracts.SlotUsecaseIface for notification tests.
type mockSlotUsecase struct{}

func (m *mockSlotUsecase) HandleAutomatedSlotGeneration(_ context.Context, _ fhir_dto.PractitionerRole) {
}
func (m *mockSlotUsecase) HandleOnDemandSlotRegeneration(_ context.Context, _ string) error {
	return nil
}
func (m *mockSlotUsecase) HandleSetUnavailabilityForMultiplePractitionerRoles(_ context.Context, _ contracts.SetUnavailabilityForMultiplePractitionerRolesInput) (*contracts.SetUnavailableOutcome, error) {
	return nil, nil
}
func (m *mockSlotUsecase) AcquireLocksForAppointment(_ context.Context, _ []fhir_dto.PractitionerRole, _, _ time.Time, _ time.Duration) (func(context.Context), error) {
	return func(_ context.Context) {}, nil
}
func (m *mockSlotUsecase) AcquireLocksForSlot(_ context.Context, _ *fhir_dto.Slot, _ time.Duration) (func(context.Context), error) {
	return func(_ context.Context) {}, nil
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

		// externalID format: appointment:{slotID}:{practitionerRoleID}:{patientID}:{invoiceID}:{healthcareServiceID}
		externalID := "appointment:deleted-slot-123:pr-456:pat-789:inv-012:hs-999"
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

		externalID := "appointment:missing-slot-456:pr-456:pat-789:inv-012:hs-999"
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

func TestHandleAppointmentPaymentNotification_ExpiredSlotFree(t *testing.T) {
	t.Run("EXPIRED callback returns nil when slot is already free", func(t *testing.T) {
		slotID := "slot-already-free"
		uc := &paymentUsecase{
			SlotFhirClient: &mockSlotFhirClient{
				slot: &fhir_dto.Slot{
					ID:     slotID,
					Status: fhir_dto.SlotStatusFree,
				},
				err: nil,
			},
			SlotUsecase: &mockSlotUsecase{},
			Log:         zap.NewNop(),
		}

		externalID := fmt.Sprintf("appointment:%s:pr-456:pat-789:inv-012:hs-999", slotID)
		err := uc.handleAppointmentPaymentNotification(
			context.Background(),
			externalID,
			requests.XenditInvoiceStatusExpired,
		)

		if err != nil {
			t.Errorf("expected nil for EXPIRED callback on already-free slot, got: %v", err)
		}
	})
}
