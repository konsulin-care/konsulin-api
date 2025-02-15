package locker

import (
	"context"
	"fmt"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/exceptions"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

var (
	lockerServiceInstance contracts.LockerService
	onceLockerService     sync.Once
)

type lockService struct {
	redisRepo contracts.RedisRepository
	Log       *zap.Logger
}

func NewLockService(repo contracts.RedisRepository, logger *zap.Logger) contracts.LockerService {
	onceLockerService.Do(func() {
		instance := &lockService{
			redisRepo: repo,
			Log:       logger,
		}
		lockerServiceInstance = instance
	})
	return lockerServiceInstance
}

func (s *lockService) TryLock(ctx context.Context, key string, expiration time.Duration) (bool, string, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	s.Log.Info("lockService.TryLock called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingRedisKey, key),
		zap.Duration(constvars.LoggingLockExpirationTimeKey, expiration),
	)

	lockValue := uuid.NewString()
	acquired, err := s.redisRepo.TrySetNX(ctx, key, lockValue, expiration)
	if err != nil {
		s.Log.Error("lockService.TryLock error calling redisRepo.TrySetNX",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return false, "", err
	}

	if !acquired {
		s.Log.Info("lockService.TryLock not acquired",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingRedisKey, key),
		)
		return false, "", nil
	}

	s.Log.Info("lockService.TryLock acquired lock",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingRedisKey, key),
		zap.String(constvars.LoggingLockValueKey, lockValue),
	)
	return true, lockValue, nil
}

func (s *lockService) Unlock(ctx context.Context, key, lockValue string) error {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	s.Log.Info("lockService.Unlock called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingRedisKey, key),
		zap.String(constvars.LoggingLockValueKey, lockValue),
	)

	storedVal, err := s.redisRepo.Get(ctx, key)
	if err != nil {
		s.Log.Error("lockService.Unlock error retrieving value from redis",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return err
	}

	if storedVal == "" {
		s.Log.Info("lockService.Unlock no lock found to release",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingRedisKey, key),
		)
		return nil
	}

	expectedValue := fmt.Sprintf("\"%s\"", lockValue)
	if storedVal != expectedValue {
		err := exceptions.ErrRedisUnlock(fmt.Errorf("lock not owned by this client"))
		s.Log.Error("lockService.Unlock lock ownership mismatch",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingLockStoredValueKey, storedVal),
			zap.String(constvars.LoggingLockExpectedValueKey, expectedValue),
			zap.Error(err),
		)
		return err
	}

	delErr := s.redisRepo.Delete(ctx, key)
	if delErr != nil {
		s.Log.Error("lockService.Unlock error deleting lock from redis",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(delErr),
		)
		return delErr
	}

	s.Log.Info("lockService.Unlock succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingRedisKey, key),
	)
	return nil
}
