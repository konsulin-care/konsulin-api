package research

import (
	"context"
	"time"

	"go.uber.org/zap"
)

// Clock supplies the current time and the wait the scheduler parks on
// between runs. Tests inject a fake clock to drive ticks deterministically.
type Clock interface {
	// Now returns the current time.
	Now() time.Time
	// After returns a channel that receives once d has elapsed.
	After(d time.Duration) <-chan time.Time
}

// systemClock is the production Clock backed by time.Now/time.After.
type systemClock struct{}

// Now returns the current wall time (time.Now).
func (systemClock) Now() time.Time { return time.Now() }

// After returns a channel receiving once d has elapsed (time.After).
func (systemClock) After(d time.Duration) <-chan time.Time { return time.After(d) }

// ExtractRunner is the extraction seam the scheduler invokes at every run;
// *Extractor satisfies it.
type ExtractRunner interface {
	// Extract runs one extraction pass.
	Extract(ctx context.Context) error
}

// Scheduler runs Extract at the next local midnight in its location, once
// per day, until its context is cancelled (BK-DATABANK-002, 00:00 local).
type Scheduler struct {
	extractor ExtractRunner
	loc       *time.Location
	clock     Clock
	log       *zap.Logger
}

// NewScheduler constructs the nightly extraction scheduler. extractor runs
// the pass at each midnight, loc is the APP_TIMEZONE location midnight is
// computed in (nil selects time.Local), clock supplies the time (nil
// selects the system clock), and log reports failed runs (nil selects a
// no-op logger). Returns the scheduler.
func NewScheduler(extractor ExtractRunner, loc *time.Location, clock Clock, log *zap.Logger) *Scheduler {
	if loc == nil {
		loc = time.Local
	}
	if clock == nil {
		clock = systemClock{}
	}
	if log == nil {
		log = zap.NewNop()
	}
	return &Scheduler{
		extractor: extractor,
		loc:       loc,
		clock:     clock,
		log:       log,
	}
}

// NextRun computes the next local midnight strictly after now in loc: the
// midnight of the following local day (a run at exactly 00:00:00 schedules
// the next day's pass).
// Parameters: now is the current time; loc is the timezone to anchor on.
// Returns the next local midnight as a time in loc; never panics.
func NextRun(now time.Time, loc *time.Location) time.Time {
	local := now.In(loc)
	return time.Date(local.Year(), local.Month(), local.Day()+1, 0, 0, 0, 0, loc)
}

// Start blocks until ctx is cancelled: it waits for the next local
// midnight, runs one extraction pass, and repeats. A failed pass is
// logged and the loop continues with the following midnight. Cancelling
// ctx stops the scheduler, including while parked on the wait.
// Parameters: ctx bounds the loop. Returns nothing; never panics.
func (s *Scheduler) Start(ctx context.Context) {
	for {
		next := NextRun(s.clock.Now(), s.loc)
		delay := next.Sub(s.clock.Now())
		select {
		case <-ctx.Done():
			return
		case <-s.clock.After(delay):
		}
		if err := s.extractor.Extract(ctx); err != nil {
			s.log.Error("research: nightly extraction failed", zap.Error(err))
		}
	}
}
