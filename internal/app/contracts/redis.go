package contracts

import (
	"context"
	"time"
)

type RedisRepository interface {
	Delete(ctx context.Context, key string) error
	Set(ctx context.Context, key string, value interface{}, exp time.Duration) error
	Get(ctx context.Context, key string) (string, error)
	Increment(ctx context.Context, key string) error
	PushToList(ctx context.Context, key string, values ...interface{}) error
	PopFromList(ctx context.Context, key string) error
	AddToSet(ctx context.Context, key string, values ...interface{}) error
	GetSetMembers(ctx context.Context, key string) ([]string, error)
	TrySetNX(ctx context.Context, key string, value interface{}, exp time.Duration) (bool, error)
}
