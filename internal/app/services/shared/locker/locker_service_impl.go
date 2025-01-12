package locker

import (
	"context"
	"fmt"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/pkg/exceptions"
	"time"

	"github.com/google/uuid"
)

type lockService struct {
	redisRepo contracts.RedisRepository
}

func NewLockService(repo contracts.RedisRepository) contracts.LockerService {
	return &lockService{
		redisRepo: repo,
	}
}

func (s *lockService) TryLock(ctx context.Context, key string, expiration time.Duration) (bool, string, error) {
	lockValue := uuid.NewString()

	acquired, err := s.redisRepo.TrySetNX(ctx, key, lockValue, expiration)
	if err != nil {
		return false, "", err
	}

	if !acquired {
		return false, "", nil
	}

	return true, lockValue, nil
}

func (s *lockService) Unlock(ctx context.Context, key, lockValue string) error {
	storedVal, err := s.redisRepo.Get(ctx, key)
	if err != nil {
		return err
	}

	if storedVal == "" {
		return nil
	}

	if storedVal != fmt.Sprintf("\"%s\"", lockValue) {
		return exceptions.ErrRedisUnlock(fmt.Errorf("lock not owned by this client"))
	}

	delErr := s.redisRepo.Delete(ctx, key)
	if delErr != nil {
		return delErr
	}
	return nil
}
