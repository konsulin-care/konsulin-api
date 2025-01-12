package redis

import (
	"context"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/pkg/exceptions"

	"time"

	"github.com/goccy/go-json"
	"github.com/redis/go-redis/v9"
)

type redisRepository struct {
	client *redis.Client
}

func NewRedisRepository(client *redis.Client) contracts.RedisRepository {
	return &redisRepository{client: client}
}

func (r *redisRepository) Delete(ctx context.Context, key string) error {
	err := r.client.Del(ctx, key).Err()
	if err != nil {
		return exceptions.ErrRedisDelete(err)
	}
	return err
}

func (r *redisRepository) Set(ctx context.Context, key string, value interface{}, exp time.Duration) error {
	jsonValue, err := json.Marshal(value)
	if err != nil {
		return exceptions.ErrCannotMarshalJSON(err)
	}

	err = r.client.Set(ctx, key, jsonValue, exp).Err()
	if err != nil {
		return exceptions.ErrRedisSet(err)
	}
	return err
}

func (r *redisRepository) Get(ctx context.Context, key string) (string, error) {
	data, err := r.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return data, nil
	} else if err != nil {
		return data, exceptions.ErrRedisGetNoData(err, key)
	}

	return data, err
}

func (r *redisRepository) Increment(ctx context.Context, key string) error {
	err := r.client.Incr(ctx, key).Err()
	if err != nil {
		return exceptions.ErrRedisIncrement(err)
	}
	return err
}

func (r *redisRepository) PushToList(ctx context.Context, key string, values ...interface{}) error {
	err := r.client.RPush(ctx, key, values...).Err()
	if err != nil {
		return exceptions.ErrRedisPushToList(err)
	}
	return err
}

func (r *redisRepository) PopFromList(ctx context.Context, key string) error {
	err := r.client.LPop(ctx, key).Err()
	if err != nil {
		return exceptions.ErrRedisPopFromList(err)
	}
	return err
}

func (r *redisRepository) AddToSet(ctx context.Context, key string, values ...interface{}) error {
	err := r.client.SAdd(ctx, key, values...).Err()
	if err != nil {
		return exceptions.ErrRedisAddToSet(err)
	}
	return err
}

func (r *redisRepository) GetSetMembers(ctx context.Context, key string) ([]string, error) {
	setMembers, err := r.client.SMembers(ctx, key).Result()
	if err != nil {
		return setMembers, exceptions.ErrRedisGetSetMembers(err)
	}
	return setMembers, err
}

func (r *redisRepository) TrySetNX(ctx context.Context, key string, value interface{}, exp time.Duration) (bool, error) {
	jsonValue, err := json.Marshal(value)
	if err != nil {
		return false, exceptions.ErrCannotMarshalJSON(err)
	}

	acquired, err := r.client.SetNX(ctx, key, jsonValue, exp).Result()
	if err != nil {
		return false, exceptions.ErrRedisSet(err)
	}
	return acquired, nil
}
