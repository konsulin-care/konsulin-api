package redis

import (
	"context"

	"konsulin-service/internal/app/models"
	"time"

	"github.com/goccy/go-json"
	"github.com/redis/go-redis/v9"
)

type redisRepository struct {
	client *redis.Client
}

func NewRedisRepository(client *redis.Client) RedisRepository {
	return &redisRepository{client: client}
}

func (r *redisRepository) CreateSession(ctx context.Context, session *models.Session) error {
	data, err := json.Marshal(session)
	if err != nil {
		return err
	}
	return r.client.Set(ctx, session.SessionID, data, time.Until(session.ExpiresAt)).Err()
}

func (r *redisRepository) GetSession(ctx context.Context, sessionID string) (*models.Session, error) {
	data, err := r.client.Get(ctx, sessionID).Result()
	if err != nil {
		return nil, err
	}
	var session models.Session
	if err := json.Unmarshal([]byte(data), &session); err != nil {
		return nil, err
	}
	return &session, nil
}

func (r *redisRepository) DeleteSession(ctx context.Context, sessionID string) error {
	return r.client.Del(ctx, sessionID).Err()
}

func (r *redisRepository) Set(ctx context.Context, key string, value interface{}, exp time.Duration) error {
	jsonValue, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return r.client.Set(ctx, key, jsonValue, exp).Err()
}

func (r *redisRepository) Get(ctx context.Context, key string) (string, error) {
	return r.client.Get(ctx, key).Result()
}

func (r *redisRepository) Increment(ctx context.Context, key string) (int64, error) {
	return r.client.Incr(ctx, key).Result()
}

func (r *redisRepository) PushToList(ctx context.Context, key string, values ...interface{}) error {
	return r.client.RPush(ctx, key, values...).Err()
}

func (r *redisRepository) PopFromList(ctx context.Context, key string) (string, error) {
	return r.client.LPop(ctx, key).Result()
}

func (r *redisRepository) AddToSet(ctx context.Context, key string, values ...interface{}) error {
	return r.client.SAdd(ctx, key, values...).Err()
}

func (r *redisRepository) GetSetMembers(ctx context.Context, key string) ([]string, error) {
	return r.client.SMembers(ctx, key).Result()
}
