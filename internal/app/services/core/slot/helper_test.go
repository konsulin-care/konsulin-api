package slot

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"konsulin-service/internal/pkg/fhir_dto"
)

// TestBuildSlotAdjustmentForAppointment pins the post-free-generation contract:
// the function takes no slot-duration/buffer params and only creates busy-unavailable
// overlap slots for the appointed window intersecting the role's working windows.
func TestBuildSlotAdjustmentForAppointment(t *testing.T) {
	loc := time.FixedZone("+07:00", 7*3600)
	role := fhir_dto.PractitionerRole{
		ID:     "role-1",
		Period: fhir_dto.Period{Start: "2026-08-10T00:00:00+07:00"},
		AvailableTime: []fhir_dto.AvailableTime{
			{DaysOfWeek: []string{"mon"}, AvailableStartTime: "09:00", AvailableEndTime: "17:00"},
		},
	}
	schedule := fhir_dto.Schedule{ID: "sched-1"}

	t.Run("creates busy-unavailable overlap for appointment inside working hours", func(t *testing.T) {
		start := time.Date(2026, time.August, 10, 10, 0, 0, 0, loc) // Monday
		end := time.Date(2026, time.August, 10, 11, 0, 0, 0, loc)

		toDelete, toCreate, err := BuildSlotAdjustmentForAppointment(role, schedule, nil, start, end, "slot-1")
		assert.NoError(t, err)
		assert.Nil(t, toDelete)
		assert.Len(t, toCreate, 1)
		got := toCreate[0]
		assert.Equal(t, fhir_dto.SlotStatusBusyUnavailable, got.Status)
		assert.Equal(t, start, got.Start)
		assert.Equal(t, end, got.End)
		assert.Equal(t, "Schedule/sched-1", got.Schedule.Reference)
	})

	t.Run("skips overlap already present for the same appointed slot", func(t *testing.T) {
		start := time.Date(2026, time.August, 10, 10, 0, 0, 0, loc)
		end := time.Date(2026, time.August, 10, 11, 0, 0, 0, loc)
		existing := fhir_dto.Slot{ID: "slot-1", Start: start, End: end}

		toDelete, toCreate, err := BuildSlotAdjustmentForAppointment(role, schedule, []fhir_dto.Slot{existing}, start, end, "slot-1")
		assert.NoError(t, err)
		assert.Nil(t, toDelete)
		assert.Empty(t, toCreate)
	})

	t.Run("returns nothing when the day has no working windows", func(t *testing.T) {
		start := time.Date(2026, time.August, 16, 10, 0, 0, 0, loc) // Sunday
		end := time.Date(2026, time.August, 16, 11, 0, 0, 0, loc)

		toDelete, toCreate, err := BuildSlotAdjustmentForAppointment(role, schedule, nil, start, end, "slot-1")
		assert.NoError(t, err)
		assert.Nil(t, toDelete)
		assert.Empty(t, toCreate)
	})
}
