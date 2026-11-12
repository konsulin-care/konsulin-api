package payments

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/pkg/fhir_dto"
	"konsulin-service/internal/pkg/fhir_http_client"
)

// The three shared test doubles below were relocated from
// appointment_payment_notification_test.go (deleted in T25B) via
// payment_usecase_impl.go (T25C). They live here because
// appointment_booking_test.go, appointment_cross_role_test.go,
// appointment_payment_test.go and settlement_test.go still depend on them.

// mockSlotFhirClient mocks contracts.SlotFhirClient for payment tests.
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

// mockSlotUsecase mocks contracts.SlotUsecaseIface for payment tests.
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

// mockAppointmentResourceClient stubs the raw FHIR HTTP operations used to
// fetch Appointment and HealthcareService resources during payment flows.
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
