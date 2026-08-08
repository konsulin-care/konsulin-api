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
	return r.executeRedis(ctx, key, "Delete", nil, r.del, exceptions.ErrRedisDelete)
}

func (r *redisRepository) Set(ctx context.Context, key string, value interface{}, exp time.Duration) error {
	requestID := r.logCalled(ctx, key, "Set", zap.Duration(constvars.LoggingRedisExpirationTimeKey, exp))

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
	requestID := r.logCalled(ctx, key, "Get")

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
	return r.executeRedis(ctx, key, "Increment", nil, r.incr, exceptions.ErrRedisIncrement)
}

// IncrementWithTTL atomically increments the key and sets TTL when first created.
func (r *redisRepository) IncrementWithTTL(ctx context.Context, key string, exp time.Duration) (int, error) {
	requestID := r.logCalled(ctx, key, "IncrementWithTTL", zap.Duration(constvars.LoggingRedisExpirationTimeKey, exp))

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
	return r.executeRedis(ctx, key, "PushToList", values, r.rpush, exceptions.ErrRedisPushToList)
}

func (r *redisRepository) PopFromList(ctx context.Context, key string) error {
	return r.executeRedis(ctx, key, "PopFromList", nil, r.lpop, exceptions.ErrRedisPopFromList)
}

func (r *redisRepository) AddToSet(ctx context.Context, key string, values ...interface{}) error {
	return r.executeRedis(ctx, key, "AddToSet", values, r.sadd, exceptions.ErrRedisAddToSet)
}

// logCalled logs the start of a redis operation with the request ID, key, and
// any extra fields, returning the request ID for later log entries.
func (r *redisRepository) logCalled(ctx context.Context, key, opName string, extra ...zap.Field) string {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	fields := []zap.Field{
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingRedisKey, key),
	}
	fields = append(fields, extra...)
	r.Log.Info("redisRepository."+opName+" called", fields...)
	return requestID
}

// executeRedis runs a single redis command with consistent request-ID and key
// logging, mapping command errors through errFactory. opName names the
// operation in the log entries; values is included in the "called" entry when
// non-nil.
func (r *redisRepository) executeRedis(ctx context.Context, key, opName string, values []interface{}, do func(context.Context, string, ...interface{}) error, errFactory func(error) *exceptions.CustomError) error {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	calledFields := []zap.Field{
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingRedisKey, key),
	}
	if values != nil {
		calledFields = append(calledFields, zap.Any(constvars.LoggingRedisValuesKey, values))
	}
	r.Log.Info("redisRepository."+opName+" called", calledFields...)

	err := do(ctx, key, values...)
	if err != nil {
		r.Log.Error("redisRepository."+opName+" error",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingRedisKey, key),
			zap.Error(err))
		return errFactory(err)
	}

	r.Log.Info("redisRepository."+opName+" succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingRedisKey, key))
	return err
}

// del runs the DEL command, ignoring the variadic values adapter.
func (r *redisRepository) del(ctx context.Context, key string, _ ...interface{}) error {
	return r.Client.Del(ctx, key).Err()
}

// incr runs the INCR command, ignoring the variadic values adapter.
func (r *redisRepository) incr(ctx context.Context, key string, _ ...interface{}) error {
	return r.Client.Incr(ctx, key).Err()
}

// lpop runs the LPOP command, ignoring the variadic values adapter.
func (r *redisRepository) lpop(ctx context.Context, key string, _ ...interface{}) error {
	return r.Client.LPop(ctx, key).Err()
}

// rpush runs the RPUSH command with the given values.
func (r *redisRepository) rpush(ctx context.Context, key string, values ...interface{}) error {
	return r.Client.RPush(ctx, key, values...).Err()
}

// sadd runs the SADD command with the given values.
func (r *redisRepository) sadd(ctx context.Context, key string, values ...interface{}) error {
	return r.Client.SAdd(ctx, key, values...).Err()
}

func (r *redisRepository) GetSetMembers(ctx context.Context, key string) ([]string, error) {
	requestID := r.logCalled(ctx, key, "GetSetMembers")

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
	requestID := r.logCalled(ctx, key, "TrySetNX", zap.Duration(constvars.LoggingRedisExpirationTimeKey, exp))

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
