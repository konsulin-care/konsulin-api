package slot

import (
	"context"
	"strings"
	"testing"
	"time"

	"konsulin-service/internal/app/config"
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

type mockSlotFhirClient struct {
	mock.Mock
}

func (m *mockSlotFhirClient) FindSlotByScheduleID(ctx context.Context, scheduleID string) ([]fhir_dto.Slot, error) {
	args := m.Called(ctx, scheduleID)
	if v := args.Get(0); v != nil {
		return v.([]fhir_dto.Slot), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockSlotFhirClient) FindSlotByScheduleAndTimeRange(ctx context.Context, scheduleID string, startTime, endTime time.Time) ([]fhir_dto.Slot, error) {
	args := m.Called(ctx, scheduleID, startTime, endTime)
	if v := args.Get(0); v != nil {
		return v.([]fhir_dto.Slot), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockSlotFhirClient) FindSlotByScheduleIDAndStatus(ctx context.Context, scheduleID, status string) ([]fhir_dto.Slot, error) {
	args := m.Called(ctx, scheduleID, status)
	if v := args.Get(0); v != nil {
		return v.([]fhir_dto.Slot), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockSlotFhirClient) FindSlotByID(ctx context.Context, slotID string) (*fhir_dto.Slot, error) {
	args := m.Called(ctx, slotID)
	if v := args.Get(0); v != nil {
		return v.(*fhir_dto.Slot), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockSlotFhirClient) CreateSlot(ctx context.Context, request *fhir_dto.Slot) (*fhir_dto.Slot, error) {
	args := m.Called(ctx, request)
	if v := args.Get(0); v != nil {
		return v.(*fhir_dto.Slot), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockSlotFhirClient) UpdateSlot(ctx context.Context, id string, slot *fhir_dto.Slot) (*fhir_dto.Slot, error) { // NOSONAR:go:S4144 testify mock idiom
	args := m.Called(ctx, id, slot)
	if v := args.Get(0); v != nil {
		return v.(*fhir_dto.Slot), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockSlotFhirClient) FindSlotsByScheduleWithQuery(ctx context.Context, scheduleID string, params contracts.SlotSearchParams) ([]fhir_dto.Slot, error) {
	args := m.Called(ctx, scheduleID, params)
	if v := args.Get(0); v != nil {
		return v.([]fhir_dto.Slot), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockSlotFhirClient) PostTransactionBundle(ctx context.Context, bundle map[string]any) (*fhir_dto.FHIRBundle, error) {
	args := m.Called(ctx, bundle)
	if v := args.Get(0); v != nil {
		return v.(*fhir_dto.FHIRBundle), args.Error(1)
	}
	return nil, args.Error(1)
}

// roleWithTodayWindow builds a schedulable role with a window covering today in loc.
func roleWithTodayWindow(loc *time.Location) fhir_dto.PractitionerRole {
	today := time.Now().In(loc)
	dayToken := strings.ToLower(today.Weekday().String()[:3])
	return fhir_dto.PractitionerRole{
		ID:           "role-1",
		Practitioner: fhir_dto.Reference{Reference: "Practitioner/prac-1"},
		Period:       fhir_dto.Period{Start: "2026-08-08T15:02:02+07:00"},
		AvailableTime: []fhir_dto.AvailableTime{{
			DaysOfWeek:         []string{dayToken},
			AvailableStartTime: "09:00:00",
			AvailableEndTime:   "17:00:00",
		}},
	}
}

// TestHandleAutomatedSlotGenerationUsesPractitionerDayLock verifies the worker
// acquires the practitioner-day lock (not the per-schedule-day lock) while topping up.
func TestHandleAutomatedSlotGenerationUsesPractitionerDayLock(t *testing.T) {
	loc := time.FixedZone("+07:00", 7*3600)
	mockSched := new(mockScheduleFhirClient)
	mockLocker := new(mockLockerService)
	mockSlots := new(mockSlotFhirClient)
	s := &SlotUsecase{
		schedules: mockSched,
		locker:    mockLocker,
		slots:     mockSlots,
		config:    &config.InternalConfig{App: config.App{SlotWindowDays: 1}},
		logger:    zap.NewNop(),
	}

	role := roleWithTodayWindow(loc)
	schedule := fhir_dto.Schedule{ID: "sched-1", Comment: `{"slotMinutes":60,"bufferMinutes":0}`}

	mockSched.On("FindScheduleByPractitionerRoleID", mock.Anything, "role-1").Return([]fhir_dto.Schedule{schedule}, nil)

	expectedKey := "slotgen:lock:practitioner:prac-1:" + time.Now().In(loc).Format("2006-01-02")
	mockLocker.On("TryLock", mock.Anything, expectedKey, 30*time.Second).Return(true, "tok-1", nil)
	mockSlots.On("FindSlotsByScheduleWithQuery", mock.Anything, "sched-1", mock.Anything).Return([]fhir_dto.Slot{}, nil)
	mockSlots.On("PostTransactionBundle", mock.Anything, mock.Anything).Return(&fhir_dto.FHIRBundle{}, nil)
	mockLocker.On("Unlock", mock.Anything, expectedKey, "tok-1").Return(nil)

	s.HandleAutomatedSlotGeneration(context.Background(), role)

	mockSched.AssertExpectations(t)
	mockLocker.AssertExpectations(t)
	mockSlots.AssertExpectations(t)
}

// TestHandleOnDemandSlotRegenerationUsesPractitionerDayLock verifies on-demand
// regeneration locks the practitioner-day window with the practitioner lock family.
func TestHandleOnDemandSlotRegenerationUsesPractitionerDayLock(t *testing.T) {
	loc := time.FixedZone("+07:00", 7*3600)
	mockPR := new(mockPractitionerRoleFhirClient)
	mockSched := new(mockScheduleFhirClient)
	mockLocker := new(mockLockerService)
	mockSlots := new(mockSlotFhirClient)
	s := &SlotUsecase{
		practitionerRoles: mockPR,
		schedules:         mockSched,
		locker:            mockLocker,
		slots:             mockSlots,
		config:            &config.InternalConfig{App: config.App{SlotWindowDays: 1}},
		logger:            zap.NewNop(),
	}

	role := roleWithTodayWindow(loc)
	role.Active = true
	schedule := fhir_dto.Schedule{ID: "sched-1", Comment: `{"slotMinutes":60,"bufferMinutes":0}`}

	mockPR.On("FindPractitionerRoleByID", mock.Anything, "role-1").Return(&role, nil)
	mockSched.On("FindScheduleByPractitionerRoleID", mock.Anything, "role-1").Return([]fhir_dto.Schedule{schedule}, nil)

	expectedKey := "slotgen:lock:practitioner:prac-1:" + time.Now().In(loc).Format("2006-01-02")
	mockLocker.On("TryLock", mock.Anything, expectedKey, 5*time.Minute).Return(true, "tok-1", nil)
	mockSlots.On("FindSlotsByScheduleWithQuery", mock.Anything, "sched-1", mock.Anything).Return([]fhir_dto.Slot{}, nil)
	mockSlots.On("PostTransactionBundle", mock.Anything, mock.Anything).Return(&fhir_dto.FHIRBundle{}, nil)
	mockLocker.On("Unlock", mock.Anything, expectedKey, "tok-1").Return(nil)

	err := s.HandleOnDemandSlotRegeneration(context.Background(), "role-1")
	assert.NoError(t, err)
	mockPR.AssertExpectations(t)
	mockSched.AssertExpectations(t)
	mockLocker.AssertExpectations(t)
	mockSlots.AssertExpectations(t)
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
	schedule := fhir_dto.Schedule{ID: "sched-1", Comment: `{"slotMinutes":60,"bufferMinutes":0}`}
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
