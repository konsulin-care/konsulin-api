package contracts

import (
	"context"
	"time"
)

type LockerService interface {
	TryLock(ctx context.Context, key string, expiration time.Duration) (bool, string, error)
	Unlock(ctx context.Context, key, lockValue string) error
	// Refresh extends the TTL of a lock if owned by lockValue
	Refresh(ctx context.Context, key, lockValue string, expiration time.Duration) error
}
