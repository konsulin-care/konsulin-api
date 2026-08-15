package payments

import (
	"context"
	"testing"
	"time"

	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/fhir_dto"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// TestFindCrossRoleOverlap verifies the booking-time guard that rejects a booking
// overlapping a non-free slot in ANY schedulable sibling schedule of the practitioner.
func TestFindCrossRoleOverlap(t *testing.T) {
	loc := time.FixedZone("+02:00", 2*3600)
	start := time.Date(2026, time.August, 13, 10, 0, 0, 0, loc)
	end := time.Date(2026, time.August, 13, 11, 0, 0, 0, loc)

	roles := []fhir_dto.PractitionerRole{
		{ID: "role-1"},
		{ID: "role-2"},
	}

	t.Run("returns sibling busy-tentative overlap", func(t *testing.T) {
		schedMock := &mockScheduleFhirClient{schedulesByRole: map[string][]fhir_dto.Schedule{
			"role-2": {{ID: "sched-2"}},
		}}
		slotMock := &mockSlotFhirClient{slotsBySchedule: func(scheduleID string, _ contracts.SlotSearchParams) ([]fhir_dto.Slot, error) {
			assert.Equal(t, "sched-2", scheduleID)
			return []fhir_dto.Slot{{ID: "slot-2", Status: fhir_dto.SlotStatusBusyTentative, Start: start, End: end}}, nil
		}}
		uc := &paymentUsecase{ScheduleFhirClient: schedMock, SlotFhirClient: slotMock, Log: zap.NewNop()}

		overlap, err := uc.findCrossRoleOverlap(context.Background(), roles, "sched-1", start, end)
		require.NoError(t, err)
		require.NotNil(t, overlap)
		assert.Equal(t, "slot-2", overlap.ID)
	})

	t.Run("returns sibling busy-unavailable overlap", func(t *testing.T) {
		schedMock := &mockScheduleFhirClient{schedulesByRole: map[string][]fhir_dto.Schedule{
			"role-2": {{ID: "sched-2"}},
		}}
		slotMock := &mockSlotFhirClient{slotsBySchedule: func(_ string, _ contracts.SlotSearchParams) ([]fhir_dto.Slot, error) {
			return []fhir_dto.Slot{{ID: "slot-2", Status: fhir_dto.SlotStatusBusyUnavailable, Start: start, End: end}}, nil
		}}
		uc := &paymentUsecase{ScheduleFhirClient: schedMock, SlotFhirClient: slotMock, Log: zap.NewNop()}

		overlap, err := uc.findCrossRoleOverlap(context.Background(), roles, "sched-1", start, end)
		require.NoError(t, err)
		require.NotNil(t, overlap)
		assert.Equal(t, fhir_dto.SlotStatusBusyUnavailable, overlap.Status)
	})

	t.Run("clean siblings return nil", func(t *testing.T) {
		schedMock := &mockScheduleFhirClient{schedulesByRole: map[string][]fhir_dto.Schedule{
			"role-2": {{ID: "sched-2"}},
		}}
		slotMock := &mockSlotFhirClient{slotsBySchedule: func(_ string, _ contracts.SlotSearchParams) ([]fhir_dto.Slot, error) {
			return []fhir_dto.Slot{{ID: "slot-free", Status: fhir_dto.SlotStatusFree, Start: start, End: end}}, nil
		}}
		uc := &paymentUsecase{ScheduleFhirClient: schedMock, SlotFhirClient: slotMock, Log: zap.NewNop()}

		overlap, err := uc.findCrossRoleOverlap(context.Background(), roles, "sched-1", start, end)
		require.NoError(t, err)
		assert.Nil(t, overlap)
	})

	t.Run("schedule-less sibling is skipped", func(t *testing.T) {
		// role-2 has no schedule (e.g. researcher role) — must not error and not query slots.
		schedMock := &mockScheduleFhirClient{schedulesByRole: map[string][]fhir_dto.Schedule{
			"role-2": {},
		}}
		queried := false
		slotMock := &mockSlotFhirClient{slotsBySchedule: func(_ string, _ contracts.SlotSearchParams) ([]fhir_dto.Slot, error) {
			queried = true
			return nil, nil
		}}
		uc := &paymentUsecase{ScheduleFhirClient: schedMock, SlotFhirClient: slotMock, Log: zap.NewNop()}

		overlap, err := uc.findCrossRoleOverlap(context.Background(), roles, "sched-1", start, end)
		require.NoError(t, err)
		assert.Nil(t, overlap)
		assert.False(t, queried, "slot query must not run for a schedule-less sibling")
	})

	t.Run("booked schedule is excluded", func(t *testing.T) {
		// role-1 owns the booked schedule; only role-2's schedule should be queried.
		schedMock := &mockScheduleFhirClient{schedulesByRole: map[string][]fhir_dto.Schedule{
			"role-1": {{ID: "sched-1"}},
			"role-2": {{ID: "sched-2"}},
		}}
		queried := map[string]bool{}
		slotMock := &mockSlotFhirClient{slotsBySchedule: func(scheduleID string, _ contracts.SlotSearchParams) ([]fhir_dto.Slot, error) {
			queried[scheduleID] = true
			return nil, nil
		}}
		uc := &paymentUsecase{ScheduleFhirClient: schedMock, SlotFhirClient: slotMock, Log: zap.NewNop()}

		overlap, err := uc.findCrossRoleOverlap(context.Background(), roles, "sched-1", start, end)
		require.NoError(t, err)
		assert.Nil(t, overlap)
		assert.False(t, queried["sched-1"], "booked schedule must not be re-checked")
		assert.True(t, queried["sched-2"])
	})

	t.Run("schedule fetch error is propagated", func(t *testing.T) {
		schedMock := &mockScheduleFhirClient{schedulesByRole: map[string][]fhir_dto.Schedule{
			"role-2": nil,
		}, err: assert.AnError}
		uc := &paymentUsecase{ScheduleFhirClient: schedMock, SlotFhirClient: &mockSlotFhirClient{}, Log: zap.NewNop()}

		_, err := uc.findCrossRoleOverlap(context.Background(), roles, "sched-1", start, end)
		assert.Error(t, err)
	})
}

// TestAcquireNotificationLocks verifies the Xendit callback resolves the
// practitionerID from the callback role and locks practitioner days.
func TestAcquireNotificationLocks(t *testing.T) {
	loc := time.FixedZone("+02:00", 2*3600)
	start := time.Date(2026, time.August, 13, 10, 0, 0, 0, loc)
	end := time.Date(2026, time.August, 13, 11, 0, 0, 0, loc)

	t.Run("locks practitioner days resolved from the callback role", func(t *testing.T) {
		roleMock := &mockPractitionerRoleFhirClient{
			role: &fhir_dto.PractitionerRole{
				Practitioner: fhir_dto.Reference{Reference: "Practitioner/prac-9"},
			},
		}
		lockMock := &mockSlotUsecase{}
		uc := &paymentUsecase{
			PractitionerRoleFhirClient: roleMock,
			SlotUsecase:                lockMock,
			Log:                        zap.NewNop(),
		}
		fields := appointmentExternalIDFields{PractitionerRoleID: "pr-456"}

		release, err := uc.acquireNotificationLocks(context.Background(), fields, &fhir_dto.Slot{Start: start, End: end}, 30*time.Second)
		require.NoError(t, err)
		release(context.Background())
		assert.Equal(t, "prac-9", lockMock.lockPractitioner)
		assert.Equal(t, start, lockMock.lockStart)
		assert.Equal(t, end, lockMock.lockEnd)
	})

	t.Run("role not found errors", func(t *testing.T) {
		uc := &paymentUsecase{
			PractitionerRoleFhirClient: &mockPractitionerRoleFhirClient{role: nil},
			SlotUsecase:                &mockSlotUsecase{},
			Log:                        zap.NewNop(),
		}
		_, err := uc.acquireNotificationLocks(context.Background(), appointmentExternalIDFields{PractitionerRoleID: "pr-456"}, &fhir_dto.Slot{Start: start, End: end}, 30*time.Second)
		assert.Error(t, err)
	})

	t.Run("role fetch error propagates", func(t *testing.T) {
		uc := &paymentUsecase{
			PractitionerRoleFhirClient: &mockPractitionerRoleFhirClient{err: assert.AnError},
			SlotUsecase:                &mockSlotUsecase{},
			Log:                        zap.NewNop(),
		}
		_, err := uc.acquireNotificationLocks(context.Background(), appointmentExternalIDFields{PractitionerRoleID: "pr-456"}, &fhir_dto.Slot{Start: start, End: end}, 30*time.Second)
		assert.Error(t, err)
	})
}

// TestAcquireBookingLocksPassesPractitionerDay is the regression test for the
// reported 409: the booking lock must be practitioner-scoped (practitionerID +
// window only) and must never resolve sibling role timezones — a practitioner with
// period-less / schedule-less sibling roles books successfully.
func TestAcquireBookingLocksPassesPractitionerDay(t *testing.T) {
	loc := time.FixedZone("+02:00", 2*3600)
	start := time.Date(2026, time.August, 13, 10, 0, 0, 0, loc)
	end := time.Date(2026, time.August, 13, 11, 0, 0, 0, loc)

	mockLocks := &mockSlotUsecase{}
	uc := &paymentUsecase{
		SlotUsecase: mockLocks,
		Log:         zap.NewNop(),
	}

	precond := &preconditionData{
		PractitionerRole: &fhir_dto.PractitionerRole{
			Practitioner: fhir_dto.Reference{Reference: constvars.FHIRRefPrefixPractitioner + "DG5CY3QAKEOXE2Y6"},
			// no Period on purpose: sibling roles may be period-less; booking must not care
		},
		Slot: &fhir_dto.Slot{Start: start, End: end},
	}

	release, err := uc.acquireBookingLocks(context.Background(), precond, 30*time.Second)
	require.NoError(t, err)
	release(context.Background())

	assert.Equal(t, "DG5CY3QAKEOXE2Y6", mockLocks.lockPractitioner)
	assert.Equal(t, start, mockLocks.lockStart)
	assert.Equal(t, end, mockLocks.lockEnd)
}

// TestAcquireBookingLocksPropagatesLockError verifies lock failures surface as errors.
func TestAcquireBookingLocksPropagatesLockError(t *testing.T) {
	loc := time.FixedZone("+02:00", 2*3600)
	uc := &paymentUsecase{
		SlotUsecase: &mockSlotUsecase{lockErr: assert.AnError},
		Log:         zap.NewNop(),
	}
	precond := &preconditionData{
		PractitionerRole: &fhir_dto.PractitionerRole{
			Practitioner: fhir_dto.Reference{Reference: "Practitioner/prac-1"},
		},
		Slot: &fhir_dto.Slot{
			Start: time.Date(2026, time.August, 13, 10, 0, 0, 0, loc),
			End:   time.Date(2026, time.August, 13, 11, 0, 0, 0, loc),
		},
	}
	_, err := uc.acquireBookingLocks(context.Background(), precond, 30*time.Second)
	assert.Error(t, err)
}
