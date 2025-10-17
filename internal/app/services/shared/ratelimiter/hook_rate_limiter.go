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

	// Monthly quota keys
	monthKey := fmt.Sprintf("HOOK:QUOTA:%s:%s", in.NowUTC.Format("200601"), service)
	actorID := strings.TrimSpace(in.ActorID)
	var monthKeyUser string
	if actorID != "" {
		monthKeyUser = fmt.Sprintf("HOOK:QUOTA_USER:%s:%s:%s", in.NowUTC.Format("200601"), service, actorID)
	}
	// TTL until the end of month (UTC)
	firstOfNextMonth := time.Date(in.NowUTC.Year(), in.NowUTC.Month()+1, 1, 0, 0, 0, 0, time.UTC)
	ttlMonthly := time.Until(firstOfNextMonth)

	// Read current monthly count (service)
	currentMonthlyStr, _ := l.redis.Get(ctx, monthKey)
	var currentMonthly int
	if currentMonthlyStr != "" {
		_ = json.Unmarshal([]byte(currentMonthlyStr), &currentMonthly)
	}
	// Read current monthly count (user)
	var currentMonthlyUser int
	if monthKeyUser != "" {
		currentMonthlyUserStr, _ := l.redis.Get(ctx, monthKeyUser)
		if currentMonthlyUserStr != "" {
			_ = json.Unmarshal([]byte(currentMonthlyUserStr), &currentMonthlyUser)
		}
	}

	if (currentMonthly >= l.monthlyQuota && l.monthlyQuota > 0) ||
		(monthKeyUser != "" && currentMonthlyUser >= l.monthlyQuota && l.monthlyQuota > 0) {
		// Over monthly quota => Retry-After until next month boundary
		return &EvaluateOutput{Allowed: false, RetryAfterSecs: int(ttlMonthly.Seconds()) + 1, LimitedByMonthly: true}, nil
	}

	// 60s window keys
	minuteKey := fmt.Sprintf("HOOK:LIMIT:%s:%s", in.NowUTC.Format("200601021504"), service)
	var minuteKeyUser string
	if actorID != "" {
		minuteKeyUser = fmt.Sprintf("HOOK:LIMIT_USER:%s:%s:%s", in.NowUTC.Format("200601021504"), service, actorID)
	}
	// TTL until end of the current minute window
	nextMinute := in.NowUTC.Truncate(time.Minute).Add(time.Minute)
	ttlMinute := time.Until(nextMinute)

	// Read current minute count (service)
	currentMinuteStr, _ := l.redis.Get(ctx, minuteKey)
	var currentMinute int
	if currentMinuteStr != "" {
		_ = json.Unmarshal([]byte(currentMinuteStr), &currentMinute)
	}
	// Read current minute count (user)
	var currentMinuteUser int
	if minuteKeyUser != "" {
		currentMinuteUserStr, _ := l.redis.Get(ctx, minuteKeyUser)
		if currentMinuteUserStr != "" {
			_ = json.Unmarshal([]byte(currentMinuteUserStr), &currentMinuteUser)
		}
	}

	if (currentMinute >= l.rateLimit && l.rateLimit > 0) ||
		(minuteKeyUser != "" && currentMinuteUser >= l.rateLimit && l.rateLimit > 0) {
		// Over per-minute window => Retry-After until next minute
		return &EvaluateOutput{Allowed: false, RetryAfterSecs: int(ttlMinute.Seconds()) + 1, LimitedByMonthly: false}, nil
	}

	// Increment counters with TTL set if first time (service)
	if currentMinute == 0 {
		_ = l.redis.Set(ctx, minuteKey, 1, ttlMinute+time.Second)
	} else {
		_ = l.redis.Increment(ctx, minuteKey)
	}

	if currentMonthly == 0 {
		_ = l.redis.Set(ctx, monthKey, 1, ttlMonthly+time.Minute)
	} else {
		_ = l.redis.Increment(ctx, monthKey)
	}

	// Increment user-specific counters
	if minuteKeyUser != "" {
		if currentMinuteUser == 0 {
			_ = l.redis.Set(ctx, minuteKeyUser, 1, ttlMinute+time.Second)
		} else {
			_ = l.redis.Increment(ctx, minuteKeyUser)
		}
	}
	if monthKeyUser != "" {
		if currentMonthlyUser == 0 {
			_ = l.redis.Set(ctx, monthKeyUser, 1, ttlMonthly+time.Minute)
		} else {
			_ = l.redis.Increment(ctx, monthKeyUser)
		}
	}

	return &EvaluateOutput{Allowed: true}, nil
}
