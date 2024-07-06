package redis

import (
	"context"
	"konsulin-service/internal/app/models"
	"time"
)

type RedisRepository interface {
	CreateSession(ctx context.Context, session *models.Session) error
	GetSession(ctx context.Context, sessionID string) (*models.Session, error)
	DeleteSession(ctx context.Context, sessionID string) error
	Set(ctx context.Context, key string, value interface{}, exp time.Duration) error
	Get(ctx context.Context, key string) (string, error)
	Increment(ctx context.Context, key string) (int64, error)
	PushToList(ctx context.Context, key string, values ...interface{}) error
	PopFromList(ctx context.Context, key string) (string, error)
	AddToSet(ctx context.Context, key string, values ...interface{}) error
	GetSetMembers(ctx context.Context, key string) ([]string, error)
}
