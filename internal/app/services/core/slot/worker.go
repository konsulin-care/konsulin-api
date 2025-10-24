package slot

import (
	"context"
	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/contracts"
	"time"

	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
)

// leaderLockKey is the fixed key used to ensure a single generator leader.
const leaderLockKey = "slotgen:leader"

// Worker periodically maintains rolling slots.
type Worker struct {
	log         *zap.Logger
	cfg         *config.InternalConfig
	locker      contracts.LockerService
	roles       contracts.PractitionerRoleFhirClient
	slotUsecase contracts.SlotUsecaseIface
	stop        chan struct{}
	cron        *cron.Cron
	runCtx      context.Context
	cancel      context.CancelFunc
}

func NewWorker(log *zap.Logger, cfg *config.InternalConfig, lockerSvc contracts.LockerService, rolesClient contracts.PractitionerRoleFhirClient, slotUsecase contracts.SlotUsecaseIface) *Worker {
	return &Worker{log: log, cfg: cfg, locker: lockerSvc, roles: rolesClient, slotUsecase: slotUsecase, stop: make(chan struct{})}
}

// Start begins the periodic loop. Returns a stop function.
func (w *Worker) Start(ctx context.Context) {
	// create run context we can cancel from Stop()
	w.runCtx, w.cancel = context.WithCancel(ctx)
	c := cron.New()
	// Use the validated cron spec from config
	spec := w.cfg.App.SlotWorkerCronSpec
	_, err := c.AddFunc(spec, func() { w.runOnce(w.runCtx) })
	if err != nil {
		w.log.Warn("slot.worker: failed to schedule with provided cron spec; falling back to @daily", zap.Error(err))
		c = cron.New()
		_, _ = c.AddFunc("@daily", func() { w.runOnce(w.runCtx) })
	}
	c.Start()
	w.cron = c
}

// Stop gracefully stops the worker cron and any in-flight runOnce refreshers.
func (w *Worker) Stop() {
	// signal run goroutines to stop
	select {
	case <-w.stop:
		// already closed
	default:
		close(w.stop)
	}
	if w.cancel != nil {
		w.cancel()
	}
	if w.cron != nil {
		ctx := w.cron.Stop() // wait for running jobs to finish
		<-ctx.Done()
	}
}

func (w *Worker) runOnce(ctx context.Context) {
	// Acquire leader lock
	ttl := 2 * time.Minute // fixed small TTL; cron cadence is independent
	acquired, token, err := w.locker.TryLock(ctx, leaderLockKey, ttl)
	if err != nil {
		w.log.Warn("slot.worker: leader lock attempt failed", zap.Error(err))
		return
	}
	if !acquired {
		w.log.Info("slot.worker: leader lock not acquired; another instance is running")
		return
	}
	defer w.locker.Unlock(ctx, leaderLockKey, token)

	// Start TTL refresher goroutine
	refreshCtx, cancelRefresh := context.WithCancel(ctx)
	defer cancelRefresh()
	go func() {
		// refresh a bit before expiry (e.g., half TTL)
		tick := time.NewTicker(ttl / 2)
		defer tick.Stop()
		for {
			select {
			case <-refreshCtx.Done():
				return
			case <-tick.C:
				w.log.Info("slot.worker: refreshing leader lock TTL", zap.String("key", leaderLockKey), zap.String("token", token), zap.Duration("ttl", ttl))
				if err := w.locker.Refresh(ctx, leaderLockKey, token, ttl); err != nil {
					w.log.Warn("slot.worker: failed to refresh leader lock TTL", zap.Error(err))
				}
			}
		}
	}()

	active := true
	roles, err := w.roles.Search(ctx, contracts.PractitionerRoleSearchParams{
		Active:   &active,
		Elements: []string{"id", "period", "availableTime"},
	})
	if err != nil {
		w.log.Warn("slot.worker: roles search failed", zap.Error(err))
		return
	}

	for _, role := range roles {
		w.slotUsecase.HandleAutomatedSlotGeneration(ctx, role)
	}
}
