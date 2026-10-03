package ratelimiter

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"testing"
	"time"

	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/contracts"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type counterRepository struct {
	contracts.RedisRepository
	values                 map[string]string
	ttls                   map[string]time.Duration
	getErrors, writeErrors map[string]error
	atomicCount            int
	atomicError            error
	atomicKey              string
	atomicTTL              time.Duration
}

func newCounterRepository() *counterRepository {
	return &counterRepository{values: map[string]string{}, ttls: map[string]time.Duration{}, getErrors: map[string]error{}, writeErrors: map[string]error{}}
}

func (r *counterRepository) Get(_ context.Context, key string) (string, error) {
	return r.values[key], r.getErrors[key]
}

func (r *counterRepository) Set(_ context.Context, key string, value interface{}, ttl time.Duration) error {
	if err := r.writeErrors[key]; err != nil {
		return err
	}
	r.values[key], r.ttls[key] = fmt.Sprint(value), ttl
	return nil
}

func (r *counterRepository) Increment(_ context.Context, key string) error {
	if err := r.writeErrors[key]; err != nil {
		return err
	}
	value, err := strconv.Atoi(r.values[key])
	if err != nil {
		return err
	}
	r.values[key] = strconv.Itoa(value + 1)
	return nil
}

func (r *counterRepository) IncrementWithTTL(_ context.Context, key string, ttl time.Duration) (int, error) {
	r.atomicKey, r.atomicTTL = key, ttl
	return r.atomicCount, r.atomicError
}

type resourceLimiterCase struct {
	name     string
	input    *ApplyResourceLimiterInput
	count    int
	redisErr error
	allowed  bool
	retry    int
	wantErr  bool
}

func TestResourceLimiter(t *testing.T) {
	for _, tc := range []resourceLimiterCase{
		{name: "nil input", wantErr: true},
		{name: "disabled quota", input: &ApplyResourceLimiterInput{}, allowed: true},
		{name: "missing resource", input: &ApplyResourceLimiterInput{LimiterGroupName: "hook", MaxQuota: 1}, retry: 60},
		{name: "missing group", input: &ApplyResourceLimiterInput{ResourceName: "api", MaxQuota: 1, WindowDurationSec: 30}, retry: 30},
		{name: "at quota", input: &ApplyResourceLimiterInput{ResourceName: " API ", LimiterGroupName: " hook ", MaxQuota: 2, WindowDurationSec: 60, NowUTC: time.Unix(125, 0).UTC()}, count: 2, allowed: true},
		{name: "over quota", input: &ApplyResourceLimiterInput{ResourceName: "api", LimiterGroupName: "hook", MaxQuota: 2, WindowDurationSec: 60, NowUTC: time.Unix(125, 0).UTC()}, count: 3, retry: 56},
		{name: "zero time uses clock", input: &ApplyResourceLimiterInput{ResourceName: "api", LimiterGroupName: "hook", MaxQuota: 2}, count: 1, allowed: true},
		{name: "redis fails closed", input: &ApplyResourceLimiterInput{ResourceName: "api", LimiterGroupName: "hook", MaxQuota: 2}, redisErr: errors.New("redis unavailable"), wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) { runResourceLimiterCase(t, tc) })
	}
}

func runResourceLimiterCase(t *testing.T, tc resourceLimiterCase) {
	repo := newCounterRepository()
	repo.atomicCount, repo.atomicError = tc.count, tc.redisErr
	limiter := NewResourceLimiter(repo, zap.NewNop())
	got, err := limiter.ApplyResourceLimiter(context.Background(), tc.input)
	if tc.wantErr {
		require.Error(t, err)
	} else {
		require.NoError(t, err)
	}
	require.Equal(t, tc.allowed, got.Allowed)
	require.Equal(t, tc.retry, got.RetryAfterSecs)
	if tc.redisErr != nil {
		require.ErrorIs(t, err, tc.redisErr)
	}
	if repo.atomicKey != "" {
		require.Positive(t, repo.atomicTTL)
		require.Contains(t, repo.atomicKey, "HOOK:api:")
	}
	if tc.name == "at quota" {
		require.Equal(t, "HOOK:api:2", repo.atomicKey)
		require.Equal(t, 61*time.Second, repo.atomicTTL)
	}
}

func TestHookRateLimiter(t *testing.T) {
	now := time.Date(2026, 12, 31, 23, 59, 30, 0, time.UTC)
	month, monthUser, monthlyTTL := buildMonthlyQuotaKeys("api", "user", now)
	minute, minuteUser, minuteTTL := buildMinuteWindowKeys("api", "user", now)
	require.Equal(t, "HOOK:QUOTA:202612:api", month)
	require.Equal(t, "HOOK:QUOTA_USER:202612:api:user", monthUser)
	require.Equal(t, "HOOK:LIMIT:202612312359:api", minute)
	require.Equal(t, "HOOK:LIMIT_USER:202612312359:api:user", minuteUser)
	require.Equal(t, 30*time.Second, monthlyTTL)
	require.Equal(t, 30*time.Second, minuteTTL)
	for _, tc := range []struct {
		name, service, actor, key, value string
		readErr, writeErr                error
		allowed, monthly, wantErr        bool
		retry                            int
	}{
		{name: "empty service", retry: 60},
		{name: "unlisted service", service: "other", allowed: true},
		{name: "new global counters", service: " API ", allowed: true},
		{name: "new actor counters", service: "api", actor: " user ", allowed: true},
		{name: "global monthly quota", service: "api", key: month, value: "5", monthly: true, retry: 31},
		{name: "actor monthly quota", service: "api", actor: "user", key: monthUser, value: "5", monthly: true, retry: 31},
		{name: "global minute quota", service: "api", key: minute, value: "2", retry: 31},
		{name: "actor minute quota", service: "api", actor: "user", key: minuteUser, value: "2", retry: 31},
		{name: "malformed monthly counter", service: "api", key: month, value: "invalid", wantErr: true},
		{name: "malformed actor monthly counter", service: "api", actor: "user", key: monthUser, value: "invalid", wantErr: true},
		{name: "malformed minute counter", service: "api", key: minute, value: "invalid", wantErr: true},
		{name: "monthly redis read failure", service: "api", key: month, readErr: errors.New("unavailable"), wantErr: true},
		{name: "redis write failure", service: "api", key: minute, writeErr: errors.New("unavailable"), wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := newCounterRepository()
			repo.values[tc.key], repo.getErrors[tc.key], repo.writeErrors[tc.key] = tc.value, tc.readErr, tc.writeErr
			cfg := &config.InternalConfig{}
			cfg.Webhook.RateLimitedServices = " API, , second "
			cfg.Webhook.RateLimit, cfg.Webhook.MonthlyQuota = 2, 5
			limiter := NewHookRateLimiter(repo, zap.NewNop(), cfg)
			got, err := limiter.Evaluate(context.Background(), &EvaluateInput{ServiceName: tc.service, ActorID: tc.actor, NowUTC: now})
			if tc.wantErr {
				require.Error(t, err)
				require.Nil(t, got)
				return
			}
			require.NoError(t, err)
			require.Equal(t, &EvaluateOutput{Allowed: tc.allowed, RetryAfterSecs: tc.retry, LimitedByMonthly: tc.monthly}, got)
			if tc.name == "new actor counters" {
				for _, key := range []string{minute, minuteUser, month, monthUser} {
					require.Equal(t, "1", repo.values[key])
				}
				require.Equal(t, 31*time.Second, repo.ttls[minute])
				require.Equal(t, 90*time.Second, repo.ttls[month])
			}
		})
	}
	limiter := NewHookRateLimiter(nil, zap.NewNop(), &config.InternalConfig{})
	_, err := limiter.Evaluate(context.Background(), nil)
	require.Error(t, err)
}

var incrementKeys = []string{"minute", "month", "minute-user", "month-user"}

// TestIncrementCountersAndWriteFailures covers the Set (current=0) and
// Increment (current=1) paths, plus a write failure on each counter key.
func TestIncrementCountersAndWriteFailures(t *testing.T) {
	for _, current := range []int{0, 1} {
		t.Run(fmt.Sprintf("current=%d success", current), func(t *testing.T) { runIncrementSuccess(t, current) })
		for _, failedKey := range incrementKeys {
			t.Run(fmt.Sprintf("current=%d failed=%s", current, failedKey), func(t *testing.T) { runIncrementFailure(t, current, failedKey) })
		}
	}
}

// newIncrementFixture seeds every counter key with current and returns the matching state.
func newIncrementFixture(current int) (*counterRepository, counterState) {
	repo := newCounterRepository()
	for _, key := range incrementKeys {
		repo.values[key] = strconv.Itoa(current)
	}
	cs := counterState{MinuteKey: "minute", MonthKey: "month", MinuteKeyUser: "minute-user", MonthKeyUser: "month-user", CurrentMinute: current, CurrentMonthly: current, CurrentMinuteUser: current, CurrentMonthlyUser: current, TTLMinute: time.Minute, TTLMonthly: time.Hour}
	return repo, cs
}

func runIncrementSuccess(t *testing.T, current int) {
	repo, cs := newIncrementFixture(current)
	require.NoError(t, incrementCounters(context.Background(), repo, cs))
	for _, key := range incrementKeys {
		require.Equal(t, strconv.Itoa(current+1), repo.values[key])
	}
}

func runIncrementFailure(t *testing.T, current int, failedKey string) {
	repo, cs := newIncrementFixture(current)
	failure := errors.New("write failed")
	repo.writeErrors[failedKey] = failure
	require.ErrorIs(t, incrementCounters(context.Background(), repo, cs), failure)
}
