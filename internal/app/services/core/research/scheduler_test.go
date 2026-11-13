package research

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testLoc is a fixed +07:00 zone standing in for APP_TIMEZONE.
var testLoc = time.FixedZone("WIB", 7*60*60)

// fakeClock serves a fixed now and one test-controlled tick channel; every
// After records the requested delay so tests can assert the schedule.
type fakeClock struct {
	mu     sync.Mutex
	now    time.Time
	tick   chan time.Time
	delays []time.Duration
}

// Now returns the fixed current time.
func (c *fakeClock) Now() time.Time {
	return c.now
}

// After records d and returns the shared tick channel.
func (c *fakeClock) After(d time.Duration) <-chan time.Time {
	c.mu.Lock()
	c.delays = append(c.delays, d)
	c.mu.Unlock()
	return c.tick
}

// recordedDelays returns the delays After was asked to wait for.
func (c *fakeClock) recordedDelays() []time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]time.Duration(nil), c.delays...)
}

// countingExtractor records each extraction pass on a signal channel.
type countingExtractor struct {
	calls chan struct{}
}

// Extract signals one pass and succeeds.
func (e *countingExtractor) Extract(_ context.Context) error {
	select {
	case e.calls <- struct{}{}:
	default:
	}
	return nil
}

// TestNextRun_IsNextLocalMidnight covers the first contract line: the next
// run is computed as the next local midnight in the given location.
func TestNextRun_IsNextLocalMidnight(t *testing.T) {
	now := time.Date(2026, 6, 15, 13, 45, 30, 123456789, testLoc)

	got := NextRun(now, testLoc)

	want := time.Date(2026, 6, 16, 0, 0, 0, 0, testLoc)
	require.Equal(t, want, got, "the next run must be the next local midnight")
	assert.Equal(t, 0, got.Hour())
	assert.Equal(t, 0, got.Minute())
	assert.Equal(t, 0, got.Second())
}

// TestNextRun_AfterLocalMidnightSkipsToFollowingDay pins that a run at
// exactly local midnight schedules the following day, not an immediate
// re-run.
func TestNextRun_AfterLocalMidnightSkipsToFollowingDay(t *testing.T) {
	midnight := time.Date(2026, 6, 16, 0, 0, 0, 0, testLoc)

	got := NextRun(midnight, testLoc)

	require.Equal(t, time.Date(2026, 6, 17, 0, 0, 0, 0, testLoc), got)
}

// TestScheduler_TickRunsExtractionAndCancelStopsScheduler covers the
// second contract line: a tick runs the extraction pass, and cancelling
// the context stops the scheduler.
func TestScheduler_TickRunsExtractionAndCancelStopsScheduler(t *testing.T) {
	now := time.Date(2026, 6, 15, 13, 45, 30, 0, testLoc)
	clk := &fakeClock{now: now, tick: make(chan time.Time)}
	ex := &countingExtractor{calls: make(chan struct{}, 1)}
	s := NewScheduler(ex, testLoc, clk, nil)

	ctx, cancel := context.WithCancel(context.Background())
	stopped := make(chan struct{})
	go func() {
		s.Start(ctx)
		close(stopped)
	}()

	// Deliver a tick: the send blocks until the scheduler is parked on the
	// wait, so the extraction pass follows the tick.
	clk.tick <- now
	select {
	case <-ex.calls:
	case <-time.After(2 * time.Second):
		t.Fatal("a tick must run the extraction pass")
	}

	// The scheduler must have waited until the next local midnight.
	delays := clk.recordedDelays()
	require.NotEmpty(t, delays)
	assert.Equal(t, time.Date(2026, 6, 16, 0, 0, 0, 0, testLoc).Sub(now), delays[0],
		"the scheduler must wait until the next local midnight")

	cancel()
	select {
	case <-stopped:
	case <-time.After(2 * time.Second):
		t.Fatal("cancelling the context must stop the scheduler")
	}
}
