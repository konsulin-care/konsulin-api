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

	currentMonthly, currentMonthlyUser, monthlyExceeded, err := checkCounter(ctx, l.redis, monthKey, monthKeyUser, l.monthlyQuota)
	if err != nil {
		return nil, err
	}
	if monthlyExceeded {
		return &EvaluateOutput{Allowed: false, RetryAfterSecs: int(ttlMonthly.Seconds()) + 1, LimitedByMonthly: true}, nil
	}

	currentMinute, currentMinuteUser, minuteExceeded, err := checkCounter(ctx, l.redis, minuteKey, minuteKeyUser, l.rateLimit)
	if err != nil {
		return nil, err
	}
	if minuteExceeded {
		return &EvaluateOutput{Allowed: false, RetryAfterSecs: int(ttlMinute.Seconds()) + 1, LimitedByMonthly: false}, nil
	}

	cs := counterState{
		MinuteKey: minuteKey, MinuteKeyUser: minuteKeyUser,
		MonthKey: monthKey, MonthKeyUser: monthKeyUser,
		CurrentMinute: currentMinute, CurrentMinuteUser: currentMinuteUser,
		CurrentMonthly: currentMonthly, CurrentMonthlyUser: currentMonthlyUser,
		TTLMinute: ttlMinute, TTLMonthly: ttlMonthly,
	}
	incrementCounters(ctx, l.redis, cs)

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

// checkCounter reads counters and returns current values plus exceeded flag.
// Used for both monthly quota and minute-window rate limiting.
func checkCounter(ctx context.Context, redis contracts.RedisRepository, key, keyUser string, limit int) (current, currentUser int, exceeded bool, err error) {
	current, err = readIntCounter(ctx, redis, key)
	if err != nil {
		return 0, 0, false, err
	}
	if limit > 0 && current >= limit {
		return current, 0, true, nil
	}

	if keyUser != "" {
		currentUser, err = readIntCounter(ctx, redis, keyUser)
		if err != nil {
			return 0, 0, false, err
		}
		if limit > 0 && currentUser >= limit {
			return current, currentUser, true, nil
		}
	}
	return current, currentUser, false, nil
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

// counterState holds the current counter values and TTLs for rate limiting keys.
type counterState struct {
	MinuteKey, MinuteKeyUser, MonthKey, MonthKeyUser                     string
	CurrentMinute, CurrentMinuteUser, CurrentMonthly, CurrentMonthlyUser int
	TTLMinute, TTLMonthly                                                time.Duration
}



// incrementCounters increments service-level and user-level counters with TTL.
func incrementCounters(ctx context.Context, redis contracts.RedisRepository, cs counterState) {
	if cs.CurrentMinute == 0 {
		_ = redis.Set(ctx, cs.MinuteKey, 1, cs.TTLMinute+time.Second)
	} else {
		_ = redis.Increment(ctx, cs.MinuteKey)
	}

	if cs.CurrentMonthly == 0 {
		_ = redis.Set(ctx, cs.MonthKey, 1, cs.TTLMonthly+time.Minute)
	} else {
		_ = redis.Increment(ctx, cs.MonthKey)
	}

	if cs.MinuteKeyUser != "" {
		if cs.CurrentMinuteUser == 0 {
			_ = redis.Set(ctx, cs.MinuteKeyUser, 1, cs.TTLMinute+time.Second)
		} else {
			_ = redis.Increment(ctx, cs.MinuteKeyUser)
		}
	}

	if cs.MonthKeyUser != "" {
		if cs.CurrentMonthlyUser == 0 {
			_ = redis.Set(ctx, cs.MonthKeyUser, 1, cs.TTLMonthly+time.Minute)
		} else {
			_ = redis.Increment(ctx, cs.MonthKeyUser)
		}
	}
}
