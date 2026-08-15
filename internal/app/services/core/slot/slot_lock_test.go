package slot

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockLockerService struct {
	mock.Mock
}

func (m *mockLockerService) TryLock(ctx context.Context, key string, expiration time.Duration) (bool, string, error) {
	args := m.Called(ctx, key, expiration)
	return args.Bool(0), args.String(1), args.Error(2)
}

func (m *mockLockerService) Unlock(ctx context.Context, key, lockValue string) error {
	return m.Called(ctx, key, lockValue).Error(0)
}

func (m *mockLockerService) Refresh(ctx context.Context, key, lockValue string, expiration time.Duration) error { // NOSONAR:go:S4144 testify mock idiom
	return m.Called(ctx, key, lockValue, expiration).Error(0)
}

// TestPractitionerDayLockKey pins the practitioner-day lock key format:
// slotgen:lock:practitioner:<practitionerID>:<YYYY-MM-DD> (no timezone suffix).
func TestPractitionerDayLockKey(t *testing.T) {
	s := &SlotUsecase{}
	day := time.Date(2026, time.August, 13, 0, 0, 0, 0, time.UTC)
	assert.Equal(t, "slotgen:lock:practitioner:prac-1:2026-08-13", s.practitionerDayLockKey("prac-1", day))
}

// TestPractitionerDayTargetsForWindow verifies local-day computation semantics.
func TestPractitionerDayTargetsForWindow(t *testing.T) {
	loc := time.FixedZone("+02:00", 2*3600)
	s := &SlotUsecase{}

	t.Run("single local day", func(t *testing.T) {
		start := time.Date(2026, time.August, 13, 9, 0, 0, 0, loc)
		end := time.Date(2026, time.August, 13, 10, 0, 0, 0, loc)
		targets := s.practitionerDayTargetsForWindow("prac-1", loc, start, end)
		assert.Len(t, targets, 1)
		assert.Equal(t, "prac-1", targets[0].PractitionerID)
		assert.Equal(t, "2026-08-13", targets[0].Day.Format("2006-01-02"))
	})

	t.Run("end exactly at midnight excludes next day", func(t *testing.T) {
		start := time.Date(2026, time.August, 13, 9, 0, 0, 0, loc)
		end := time.Date(2026, time.August, 14, 0, 0, 0, 0, loc)
		targets := s.practitionerDayTargetsForWindow("prac-1", loc, start, end)
		assert.Len(t, targets, 1)
	})

	t.Run("multi-day window", func(t *testing.T) {
		start := time.Date(2026, time.August, 13, 22, 0, 0, 0, loc)
		end := time.Date(2026, time.August, 15, 10, 0, 0, 0, loc)
		targets := s.practitionerDayTargetsForWindow("prac-1", loc, start, end)
		assert.Len(t, targets, 3)
	})

	t.Run("inverted window returns nil", func(t *testing.T) {
		start := time.Date(2026, time.August, 13, 10, 0, 0, 0, loc)
		end := time.Date(2026, time.August, 13, 9, 0, 0, 0, loc)
		assert.Nil(t, s.practitionerDayTargetsForWindow("prac-1", loc, start, end))
	})

	t.Run("window crossing UTC date boundary uses local day", func(t *testing.T) {
		start := time.Date(2026, time.August, 13, 23, 30, 0, 0, loc)
		end := time.Date(2026, time.August, 14, 0, 30, 0, 0, loc)
		targets := s.practitionerDayTargetsForWindow("prac-1", loc, start, end)
		assert.Len(t, targets, 2)
		assert.Equal(t, "2026-08-13", targets[0].Day.Format("2006-01-02"))
		assert.Equal(t, "2026-08-14", targets[1].Day.Format("2006-01-02"))
	})
}

// TestSortPractitionerDayTargets pins deterministic ordering (practitioner, then day).
func TestSortPractitionerDayTargets(t *testing.T) {
	targets := []practitionerDayLockTarget{
		{PractitionerID: "prac-b", Day: time.Date(2026, time.August, 14, 0, 0, 0, 0, time.UTC)},
		{PractitionerID: "prac-a", Day: time.Date(2026, time.August, 15, 0, 0, 0, 0, time.UTC)},
		{PractitionerID: "prac-a", Day: time.Date(2026, time.August, 13, 0, 0, 0, 0, time.UTC)},
	}
	sortPractitionerDayTargets(targets)
	assert.Equal(t, "prac-a", targets[0].PractitionerID)
	assert.Equal(t, "2026-08-13", targets[0].Day.Format("2006-01-02"))
	assert.Equal(t, "prac-a", targets[1].PractitionerID)
	assert.Equal(t, "2026-08-15", targets[1].Day.Format("2006-01-02"))
	assert.Equal(t, "prac-b", targets[2].PractitionerID)
}

// TestDedupePractitionerDayTargets verifies dedup by (practitioner, day) only —
// same practitioner+day from different windows collapses to one target.
func TestDedupePractitionerDayTargets(t *testing.T) {
	seen := make(map[string]struct{})
	var out []practitionerDayLockTarget
	out = dedupePractitionerDayTargets(seen, out, practitionerDayLockTarget{PractitionerID: "prac-1", Day: time.Date(2026, time.August, 13, 0, 0, 0, 0, time.UTC)})
	out = dedupePractitionerDayTargets(seen, out, practitionerDayLockTarget{PractitionerID: "prac-1", Day: time.Date(2026, time.August, 13, 0, 0, 0, 0, time.UTC)})
	out = dedupePractitionerDayTargets(seen, out, practitionerDayLockTarget{PractitionerID: "prac-1", Day: time.Date(2026, time.August, 14, 0, 0, 0, 0, time.UTC)})
	out = dedupePractitionerDayTargets(seen, out, practitionerDayLockTarget{PractitionerID: "prac-2", Day: time.Date(2026, time.August, 13, 0, 0, 0, 0, time.UTC)})
	assert.Len(t, out, 3)
}

// TestAcquireLocksForPractitionerDay verifies the booking lock acquires the
// practitioner-day key for the window, computed from the window's own location
// (booked slot offset) — sibling role timezones are never resolved.
func TestAcquireLocksForPractitionerDay(t *testing.T) {
	loc := time.FixedZone("+02:00", 2*3600)
	mockLocker := new(mockLockerService)
	s := &SlotUsecase{locker: mockLocker}

	start := time.Date(2026, time.August, 13, 10, 0, 0, 0, loc)
	end := time.Date(2026, time.August, 13, 11, 0, 0, 0, loc)
	key := "slotgen:lock:practitioner:prac-1:2026-08-13"

	mockLocker.On("TryLock", mock.Anything, key, 30*time.Second).Return(true, "t1", nil)
	mockLocker.On("Unlock", mock.Anything, key, "t1").Return(nil)

	release, err := s.AcquireLocksForPractitionerDay(context.Background(), "prac-1", start, end, 30*time.Second)
	assert.NoError(t, err)
	release(context.Background())
	mockLocker.AssertExpectations(t)
}

// TestAcquireLocksForPractitionerDayRejectsZeroWindow guards against a degenerate window.
func TestAcquireLocksForPractitionerDayRejectsZeroWindow(t *testing.T) {
	s := &SlotUsecase{locker: new(mockLockerService)}
	release, err := s.AcquireLocksForPractitionerDay(context.Background(), "prac-1", time.Time{}, time.Time{}, 30*time.Second)
	assert.Error(t, err)
	assert.Nil(t, release, "release must be nil when lock acquisition fails")
}

// TestAcquirePractitionerDayLocksOrdered verifies ordered acquisition and
// release-on-failure of the multi-day lock acquisition.
func TestAcquirePractitionerDayLocksOrdered(t *testing.T) {
	t.Run("acquires all in deterministic order and releases in reverse", func(t *testing.T) {
		mockLocker := new(mockLockerService)
		s := &SlotUsecase{locker: mockLocker}
		targets := []practitionerDayLockTarget{
			{PractitionerID: "prac-1", Day: time.Date(2026, time.August, 13, 0, 0, 0, 0, time.UTC)},
			{PractitionerID: "prac-1", Day: time.Date(2026, time.August, 14, 0, 0, 0, 0, time.UTC)},
		}
		mockLocker.On("TryLock", mock.Anything, "slotgen:lock:practitioner:prac-1:2026-08-13", 30*time.Second).Return(true, "t1", nil)
		mockLocker.On("TryLock", mock.Anything, "slotgen:lock:practitioner:prac-1:2026-08-14", 30*time.Second).Return(true, "t2", nil)
		mockLocker.On("Unlock", mock.Anything, "slotgen:lock:practitioner:prac-1:2026-08-14", "t2").Return(nil)
		mockLocker.On("Unlock", mock.Anything, "slotgen:lock:practitioner:prac-1:2026-08-13", "t1").Return(nil)

		release, err := s.acquirePractitionerDayLocksOrdered(context.Background(), targets, 30*time.Second)
		assert.NoError(t, err)
		release(context.Background())
		mockLocker.AssertExpectations(t)
	})

	t.Run("releases earlier locks when one fails", func(t *testing.T) {
		mockLocker := new(mockLockerService)
		s := &SlotUsecase{locker: mockLocker}
		targets := []practitionerDayLockTarget{
			{PractitionerID: "prac-1", Day: time.Date(2026, time.August, 13, 0, 0, 0, 0, time.UTC)},
			{PractitionerID: "prac-1", Day: time.Date(2026, time.August, 14, 0, 0, 0, 0, time.UTC)},
		}
		acquireCtx, cancel := context.WithCancel(context.Background())
		defer cancel()
		mockLocker.On("TryLock", mock.Anything, "slotgen:lock:practitioner:prac-1:2026-08-13", 30*time.Second).Return(true, "t1", nil)
		mockLocker.On("TryLock", mock.Anything, "slotgen:lock:practitioner:prac-1:2026-08-14", 30*time.Second).Return(false, "", nil)
		mockLocker.On("Unlock", mock.MatchedBy(func(ctx context.Context) bool { return ctx == acquireCtx }), "slotgen:lock:practitioner:prac-1:2026-08-13", "t1").Return(nil)

		release, err := s.acquirePractitionerDayLocksOrdered(acquireCtx, targets, 30*time.Second)
		assert.Error(t, err)
		assert.Nil(t, release, "release must be nil when acquisition fails")
		assert.Contains(t, err.Error(), "failed to acquire lock")
		mockLocker.AssertExpectations(t)
	})
}
