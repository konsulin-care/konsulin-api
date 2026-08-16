package slot

import (
	"context"
	"testing"
	"time"

	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/pkg/fhir_dto"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
)

type mockScheduleFhirClient struct {
	mock.Mock
}

func (m *mockScheduleFhirClient) CreateSchedule(ctx context.Context, request *fhir_dto.Schedule) (*fhir_dto.Schedule, error) {
	args := m.Called(ctx, request)
	if v := args.Get(0); v != nil {
		return v.(*fhir_dto.Schedule), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockScheduleFhirClient) FindScheduleByPractitionerID(ctx context.Context, practitionerID string) ([]fhir_dto.Schedule, error) {
	args := m.Called(ctx, practitionerID)
	if v := args.Get(0); v != nil {
		return v.([]fhir_dto.Schedule), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockScheduleFhirClient) FindScheduleByPractitionerRoleID(ctx context.Context, practitionerRoleID string) ([]fhir_dto.Schedule, error) {
	args := m.Called(ctx, practitionerRoleID)
	if v := args.Get(0); v != nil {
		return v.([]fhir_dto.Schedule), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockScheduleFhirClient) Search(ctx context.Context, params contracts.ScheduleSearchParams) ([]fhir_dto.Schedule, error) {
	args := m.Called(ctx, params)
	if v := args.Get(0); v != nil {
		return v.([]fhir_dto.Schedule), args.Error(1)
	}
	return nil, args.Error(1)
}

// TestResolveAndLockWindowsUsesPractitionerDayLocks verifies set-unavailability
// locks the practitioner's days (union across roles) with the practitioner key family.
func TestResolveAndLockWindowsUsesPractitionerDayLocks(t *testing.T) {
	loc := time.FixedZone("+07:00", 7*3600)
	mockSched := new(mockScheduleFhirClient)
	mockLocker := new(mockLockerService)
	s := &SlotUsecase{schedules: mockSched, locker: mockLocker, logger: zap.NewNop()}

	roles := []fhir_dto.PractitionerRole{{
		ID:           "role-1",
		Practitioner: fhir_dto.Reference{Reference: "Practitioner/prac-1"},
		Period:       fhir_dto.Period{Start: "2026-08-08T15:02:02+07:00"},
	}}
	schedule := fhir_dto.Schedule{ID: "sched-1"}
	mockSched.On("FindScheduleByPractitionerRoleID", mock.Anything, "role-1").Return([]fhir_dto.Schedule{schedule}, nil)

	input := contracts.SetUnavailabilityForMultiplePractitionerRolesInput{
		StartTime: time.Date(2026, time.August, 13, 9, 0, 0, 0, loc),
		EndTime:   time.Date(2026, time.August, 13, 10, 0, 0, 0, loc),
	}

	expectedKey := "slotgen:lock:practitioner:prac-1:2026-08-13"
	mockLocker.On("TryLock", mock.Anything, expectedKey, 30*time.Second).Return(true, "tok-1", nil)
	mockLocker.On("Unlock", mock.Anything, expectedKey, "tok-1").Return(nil)

	res, err := s.resolveAndLockWindows(context.Background(), roles, input)
	assert.NoError(t, err)
	assert.NotNil(t, res)
	res.release(context.Background())
	mockSched.AssertExpectations(t)
	mockLocker.AssertExpectations(t)
}
