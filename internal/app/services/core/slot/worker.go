package slot

import (
	"context"
	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/contracts"
	"time"

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
}

func NewWorker(log *zap.Logger, cfg *config.InternalConfig, lockerSvc contracts.LockerService, rolesClient contracts.PractitionerRoleFhirClient, slotUsecase contracts.SlotUsecaseIface) *Worker {
	return &Worker{log: log, cfg: cfg, locker: lockerSvc, roles: rolesClient, slotUsecase: slotUsecase, stop: make(chan struct{})}
}

// Start begins the periodic loop. Returns a stop function.
func (w *Worker) Start(ctx context.Context) (stop func()) {
	interval := time.Duration(w.cfg.App.SlotWorkerIntervalInMinutes) * time.Minute

	ticker := time.NewTicker(interval)
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				return
			case <-w.stop:
				ticker.Stop()
				return
			case <-ticker.C:
				w.runOnce(ctx)
			}
		}
	}()
	return func() { close(w.stop); <-stopped }
}

func (w *Worker) runOnce(ctx context.Context) {
	// Acquire leader lock
	ttlMinutes := w.cfg.App.SlotWorkerIntervalInMinutes
	ttl := time.Duration(ttlMinutes) * time.Minute
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
