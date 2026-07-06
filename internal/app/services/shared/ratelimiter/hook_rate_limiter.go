package ratelimiter

import (
	"context"
	"encoding/json"
	"fmt"
	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/pkg/constvars"
	"strings"
	"time"

	"go.uber.org/zap"
)

// HookRateLimiter enforces 60s window and monthly quotas by service name.
// It operates only when serviceName is listed in the blacklist CSV controllable from env.
type HookRateLimiter struct {
	redis           contracts.RedisRepository
	log             *zap.Logger
	rateLimit       int
	monthlyQuota    int
	limitedServices map[string]struct{}
}

// NewHookRateLimiter constructs the limiter using InternalConfig.Webhook.
func NewHookRateLimiter(redis contracts.RedisRepository, log *zap.Logger, cfg *config.InternalConfig) *HookRateLimiter {
	limited := make(map[string]struct{})
	if csv := strings.TrimSpace(cfg.Webhook.RateLimitedServices); csv != "" {
		for _, s := range strings.Split(csv, ",") {
			name := strings.TrimSpace(s)
			if name != "" {
				limited[strings.ToLower(name)] = struct{}{}
			}
		}
	}
	return &HookRateLimiter{
		redis:           redis,
		log:             log,
		rateLimit:       cfg.Webhook.RateLimit,
		monthlyQuota:    cfg.Webhook.MonthlyQuota,
		limitedServices: limited,
	}
}

// EvaluateInput to check rate limits for a service.
type EvaluateInput struct {
	ServiceName string
	NowUTC      time.Time
	// ActorID identifies the requester (uid, api-key-superadmin, or "anonymous")
	ActorID string
}

// EvaluateOutput contains allow flag and retry-after seconds and reason.
type EvaluateOutput struct {
	Allowed          bool
	RetryAfterSecs   int
	LimitedByMonthly bool
}

// Evaluate returns allowance; if not allowed, it returns the Retry-After seconds.
// Keys are based on service name only per requirement.
func (l *HookRateLimiter) Evaluate(ctx context.Context, in *EvaluateInput) (*EvaluateOutput, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	l.log.Info("HookRateLimiter.Evaluate called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String("service_name", in.ServiceName),
		zap.String("actor_id", in.ActorID))

	service := strings.ToLower(strings.TrimSpace(in.ServiceName))
	if service == "" {
		return &EvaluateOutput{Allowed: false, RetryAfterSecs: 60}, nil
	}

	if _, ok := l.limitedServices[service]; !ok {
		return &EvaluateOutput{Allowed: true}, nil
	}

	actorID := strings.TrimSpace(in.ActorID)

	monthKey, monthKeyUser, ttlMonthly := buildMonthlyQuotaKeys(service, actorID, in.NowUTC)
	minuteKey, minuteKeyUser, ttlMinute := buildMinuteWindowKeys(service, actorID, in.NowUTC)

	currentMonthly, currentMonthlyUser, monthlyExceeded, err := checkMonthlyQuota(ctx, l.redis, monthKey, monthKeyUser, l.monthlyQuota)
	if err != nil {
		return nil, err
	}
	if monthlyExceeded {
		return &EvaluateOutput{Allowed: false, RetryAfterSecs: int(ttlMonthly.Seconds()) + 1, LimitedByMonthly: true}, nil
	}

	currentMinute, currentMinuteUser, minuteExceeded, err := checkMinuteWindow(ctx, l.redis, minuteKey, minuteKeyUser, l.rateLimit)
	if err != nil {
		return nil, err
	}
	if minuteExceeded {
		return &EvaluateOutput{Allowed: false, RetryAfterSecs: int(ttlMinute.Seconds()) + 1, LimitedByMonthly: false}, nil
	}

	incrementCounters(ctx, l.redis, minuteKey, minuteKeyUser, monthKey, monthKeyUser, ttlMinute, ttlMonthly, currentMinute, currentMinuteUser, currentMonthly, currentMonthlyUser)

	return &EvaluateOutput{Allowed: true}, nil
}

// buildMonthlyQuotaKeys generates Redis keys and TTL for monthly quota tracking.
func buildMonthlyQuotaKeys(service, actorID string, nowUTC time.Time) (monthKey, monthKeyUser string, ttl time.Duration) {
	monthKey = fmt.Sprintf("HOOK:QUOTA:%s:%s", nowUTC.Format("200601"), service)
	if actorID != "" {
		monthKeyUser = fmt.Sprintf("HOOK:QUOTA_USER:%s:%s:%s", nowUTC.Format("200601"), service, actorID)
	}
	firstOfNextMonth := time.Date(nowUTC.Year(), nowUTC.Month()+1, 1, 0, 0, 0, 0, time.UTC)
	ttl = time.Until(firstOfNextMonth)
	return
}

// buildMinuteWindowKeys generates Redis keys and TTL for 60s window tracking.
func buildMinuteWindowKeys(service, actorID string, nowUTC time.Time) (minuteKey, minuteKeyUser string, ttl time.Duration) {
	minuteKey = fmt.Sprintf("HOOK:LIMIT:%s:%s", nowUTC.Format("200601021504"), service)
	if actorID != "" {
		minuteKeyUser = fmt.Sprintf("HOOK:LIMIT_USER:%s:%s:%s", nowUTC.Format("200601021504"), service, actorID)
	}
	nextMinute := nowUTC.Truncate(time.Minute).Add(time.Minute)
	ttl = time.Until(nextMinute)
	return
}

// checkMonthlyQuota reads counters and returns current values plus exceeded flag.
func checkMonthlyQuota(ctx context.Context, redis contracts.RedisRepository, monthKey, monthKeyUser string, monthlyQuota int) (currentMonthly, currentMonthlyUser int, exceeded bool, err error) {
	currentMonthly, err = readIntCounter(ctx, redis, monthKey)
	if err != nil {
		return 0, 0, false, err
	}
	if monthlyQuota > 0 && currentMonthly >= monthlyQuota {
		return currentMonthly, 0, true, nil
	}

	if monthKeyUser != "" {
		currentMonthlyUser, err = readIntCounter(ctx, redis, monthKeyUser)
		if err != nil {
			return 0, 0, false, err
		}
		if monthlyQuota > 0 && currentMonthlyUser >= monthlyQuota {
			return currentMonthly, currentMonthlyUser, true, nil
		}
	}
	return currentMonthly, currentMonthlyUser, false, nil
}

// checkMinuteWindow reads counters and returns current values plus exceeded flag.
func checkMinuteWindow(ctx context.Context, redis contracts.RedisRepository, minuteKey, minuteKeyUser string, rateLimit int) (currentMinute, currentMinuteUser int, exceeded bool, err error) {
	currentMinute, err = readIntCounter(ctx, redis, minuteKey)
	if err != nil {
		return 0, 0, false, err
	}
	if rateLimit > 0 && currentMinute >= rateLimit {
		return currentMinute, 0, true, nil
	}

	if minuteKeyUser != "" {
		currentMinuteUser, err = readIntCounter(ctx, redis, minuteKeyUser)
		if err != nil {
			return 0, 0, false, err
		}
		if rateLimit > 0 && currentMinuteUser >= rateLimit {
			return currentMinute, currentMinuteUser, true, nil
		}
	}
	return currentMinute, currentMinuteUser, false, nil
}

// readIntCounter reads a JSON-stored integer counter from Redis.
func readIntCounter(ctx context.Context, redis contracts.RedisRepository, key string) (int, error) {
	str, _ := redis.Get(ctx, key)
	if str == "" {
		return 0, nil
	}
	var val int
	if err := json.Unmarshal([]byte(str), &val); err != nil {
		return 0, err
	}
	return val, nil
}

// incrementCounters increments service-level and user-level counters with TTL.
func incrementCounters(ctx context.Context, redis contracts.RedisRepository, minuteKey, minuteKeyUser, monthKey, monthKeyUser string, ttlMinute, ttlMonthly time.Duration, currentMinute, currentMinuteUser, currentMonthly, currentMonthlyUser int) {
	if currentMinute == 0 {
		_ = redis.Set(ctx, minuteKey, 1, ttlMinute+time.Second)
	} else {
		_ = redis.Increment(ctx, minuteKey)
	}

	if currentMonthly == 0 {
		_ = redis.Set(ctx, monthKey, 1, ttlMonthly+time.Minute)
	} else {
		_ = redis.Increment(ctx, monthKey)
	}

	if minuteKeyUser != "" {
		if currentMinuteUser == 0 {
			_ = redis.Set(ctx, minuteKeyUser, 1, ttlMinute+time.Second)
		} else {
			_ = redis.Increment(ctx, minuteKeyUser)
		}
	}

	if monthKeyUser != "" {
		if currentMonthlyUser == 0 {
			_ = redis.Set(ctx, monthKeyUser, 1, ttlMonthly+time.Minute)
		} else {
			_ = redis.Increment(ctx, monthKeyUser)
		}
	}
}
