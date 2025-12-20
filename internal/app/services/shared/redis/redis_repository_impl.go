package redis

import (
	"context"
	"fmt"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/exceptions"
	"sync"

	"time"

	"github.com/goccy/go-json"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

var (
	redisRepositoryInstance contracts.RedisRepository
	onceRedisRepository     sync.Once
)

type redisRepository struct {
	Client *redis.Client
	Log    *zap.Logger
}

func NewRedisRepository(Client *redis.Client, Logger *zap.Logger) contracts.RedisRepository {
	onceRedisRepository.Do(func() {
		instance := &redisRepository{
			Client: Client,
			Log:    Logger,
		}
		redisRepositoryInstance = instance
	})
	return redisRepositoryInstance
}

func (r *redisRepository) Delete(ctx context.Context, key string) error {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	r.Log.Info("redisRepository.Delete called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingRedisKey, key))

	err := r.Client.Del(ctx, key).Err()
	if err != nil {
		r.Log.Error("redisRepository.Delete error",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingRedisKey, key),
			zap.Error(err))
		return exceptions.ErrRedisDelete(err)
	}

	r.Log.Info("redisRepository.Delete succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingRedisKey, key))
	return err
}

func (r *redisRepository) Set(ctx context.Context, key string, value interface{}, exp time.Duration) error {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	r.Log.Info("redisRepository.Set called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingRedisKey, key),
		zap.Duration(constvars.LoggingRedisExpirationTimeKey, exp))

	jsonValue, err := json.Marshal(value)
	if err != nil {
		r.Log.Error("redisRepository.Set error marshaling JSON",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingRedisKey, key),
			zap.Error(err))
		return exceptions.ErrCannotMarshalJSON(err)
	}

	err = r.Client.Set(ctx, key, jsonValue, exp).Err()
	if err != nil {
		r.Log.Error("redisRepository.Set error",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingRedisKey, key),
			zap.Error(err))
		return exceptions.ErrRedisSet(err)
	}

	r.Log.Info("redisRepository.Set succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingRedisKey, key))
	return err
}

func (r *redisRepository) Get(ctx context.Context, key string) (string, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	r.Log.Info("redisRepository.Get called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingRedisKey, key))

	data, err := r.Client.Get(ctx, key).Result()
	if err == redis.Nil {
		r.Log.Info("redisRepository.Get no data found",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingRedisKey, key))
		return data, nil
	} else if err != nil {
		r.Log.Error("redisRepository.Get error",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingRedisKey, key),
			zap.Error(err))
		return data, exceptions.ErrRedisGetNoData(err, key)
	}

	r.Log.Info("redisRepository.Get succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingRedisKey, key),
	)
	return data, err
}

func (r *redisRepository) Increment(ctx context.Context, key string) error {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	r.Log.Info("redisRepository.Increment called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingRedisKey, key))

	err := r.Client.Incr(ctx, key).Err()
	if err != nil {
		r.Log.Error("redisRepository.Increment error",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingRedisKey, key),
			zap.Error(err))
		return exceptions.ErrRedisIncrement(err)
	}

	r.Log.Info("redisRepository.Increment succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingRedisKey, key))
	return err
}

// IncrementWithTTL atomically increments the key and sets TTL when first created.
func (r *redisRepository) IncrementWithTTL(ctx context.Context, key string, exp time.Duration) (int, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	r.Log.Info("redisRepository.IncrementWithTTL called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingRedisKey, key),
		zap.Duration(constvars.LoggingRedisExpirationTimeKey, exp))

	script := redis.NewScript(`
		local v = redis.call("INCR", KEYS[1])
		if v == 1 then
			redis.call("PEXPIRE", KEYS[1], ARGV[1])
		end
		return v
	`)

	res, err := script.Run(ctx, r.Client, []string{key}, exp.Milliseconds()).Result()
	if err != nil {
		r.Log.Error("redisRepository.IncrementWithTTL error",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingRedisKey, key),
			zap.Error(err))
		return 0, exceptions.ErrRedisIncrement(err)
	}

	val, ok := res.(int64)
	if !ok {
		r.Log.Error("redisRepository.IncrementWithTTL unexpected result type",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingRedisKey, key),
			zap.Any("result", res))
		return 0, exceptions.ErrRedisIncrement(fmt.Errorf("unexpected result type %T", res))
	}

	r.Log.Info("redisRepository.IncrementWithTTL succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingRedisKey, key),
		zap.Int64("new_value", val))
	return int(val), nil
}

func (r *redisRepository) PushToList(ctx context.Context, key string, values ...interface{}) error {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	r.Log.Info("redisRepository.PushToList called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingRedisKey, key),
		zap.Any(constvars.LoggingRedisValuesKey, values))

	err := r.Client.RPush(ctx, key, values...).Err()
	if err != nil {
		r.Log.Error("redisRepository.PushToList error",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingRedisKey, key),
			zap.Error(err))
		return exceptions.ErrRedisPushToList(err)
	}

	r.Log.Info("redisRepository.PushToList succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingRedisKey, key))
	return err
}

func (r *redisRepository) PopFromList(ctx context.Context, key string) error {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	r.Log.Info("redisRepository.PopFromList called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingRedisKey, key))

	err := r.Client.LPop(ctx, key).Err()
	if err != nil {
		r.Log.Error("redisRepository.PopFromList error",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingRedisKey, key),
			zap.Error(err))
		return exceptions.ErrRedisPopFromList(err)
	}

	r.Log.Info("redisRepository.PopFromList succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingRedisKey, key))
	return err
}

func (r *redisRepository) AddToSet(ctx context.Context, key string, values ...interface{}) error {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	r.Log.Info("redisRepository.AddToSet called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingRedisKey, key),
		zap.Any(constvars.LoggingRedisValuesKey, values))

	err := r.Client.SAdd(ctx, key, values...).Err()
	if err != nil {
		r.Log.Error("redisRepository.AddToSet error",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingRedisKey, key),
			zap.Error(err))
		return exceptions.ErrRedisAddToSet(err)
	}

	r.Log.Info("redisRepository.AddToSet succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingRedisKey, key))
	return err
}

func (r *redisRepository) GetSetMembers(ctx context.Context, key string) ([]string, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	r.Log.Info("redisRepository.GetSetMembers called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingRedisKey, key))

	setMembers, err := r.Client.SMembers(ctx, key).Result()
	if err != nil {
		r.Log.Error("redisRepository.GetSetMembers error",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingRedisKey, key),
			zap.Error(err))
		return setMembers, exceptions.ErrRedisGetSetMembers(err)
	}

	r.Log.Info("redisRepository.GetSetMembers succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingRedisKey, key),
		zap.Int(constvars.LoggingRedisMembersKey, len(setMembers)))
	return setMembers, err
}

func (r *redisRepository) TrySetNX(ctx context.Context, key string, value interface{}, exp time.Duration) (bool, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	r.Log.Info("redisRepository.TrySetNX called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingRedisKey, key),
		zap.Duration(constvars.LoggingRedisExpirationTimeKey, exp))

	jsonValue, err := json.Marshal(value)
	if err != nil {
		r.Log.Error("redisRepository.TrySetNX error marshaling JSON",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingRedisKey, key),
			zap.Error(err))
		return false, exceptions.ErrCannotMarshalJSON(err)
	}

	acquired, err := r.Client.SetNX(ctx, key, jsonValue, exp).Result()
	if err != nil {
		r.Log.Error("redisRepository.TrySetNX error",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingRedisKey, key),
			zap.Error(err))
		return false, exceptions.ErrRedisSet(err)
	}
	r.Log.Info("redisRepository.TrySetNX succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingRedisKey, key),
		zap.Bool(constvars.LoggingRedisAcquiredKey, acquired))
	return acquired, nil
}
