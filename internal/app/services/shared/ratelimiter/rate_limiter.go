package ratelimiter

import (
	"context"
	"fmt"
	"konsulin-service/internal/app/contracts"
	"strings"
	"time"

	"go.uber.org/zap"
)

// ResourceLimiter provides a simple fixed-window limiter reusable across resources.
// Algorithm: fixed window counter stored in Redis with TTL equal to the window duration.
type ResourceLimiter struct {
	redis contracts.RedisRepository
	log   *zap.Logger
}

// NewResourceLimiter constructs a ResourceLimiter.
func NewResourceLimiter(redis contracts.RedisRepository, log *zap.Logger) *ResourceLimiter {
	return &ResourceLimiter{redis: redis, log: log}
}

// ApplyResourceLimiterInput configures limiter evaluation.
type ApplyResourceLimiterInput struct {
	// ResourceName is the entity to be limited (e.g., service name).
	ResourceName string
	// LimiterGroupName namespaces the limiter key (e.g., hook-sync).
	LimiterGroupName string
	// WindowDurationSec defines the fixed window length in seconds.
	WindowDurationSec int
	// MaxQuota is the max number of requests allowed within the window.
	MaxQuota int
	// NowUTC is optional; if zero, time.Now().UTC() is used (useful for tests).
	NowUTC time.Time
}

// ApplyResourceLimiterOutput reports allowance and retry-after seconds.
type ApplyResourceLimiterOutput struct {
	Allowed        bool
	RetryAfterSecs int
}

// ApplyResourceLimiter enforces a fixed-window limit keyed by group + resource.
// It returns Allowed=false with RetryAfterSecs until the next window boundary when quota is exceeded.
func (l *ResourceLimiter) ApplyResourceLimiter(ctx context.Context, in *ApplyResourceLimiterInput) (*ApplyResourceLimiterOutput, error) {
	if in == nil {
		return &ApplyResourceLimiterOutput{Allowed: false, RetryAfterSecs: 0}, fmt.Errorf("nil input")
	}

	resource := strings.ToLower(strings.TrimSpace(in.ResourceName))
	group := strings.ToUpper(strings.TrimSpace(in.LimiterGroupName))
	windowSec := in.WindowDurationSec
	maxQuota := in.MaxQuota
	if windowSec <= 0 {
		windowSec = 60
	}
	if maxQuota <= 0 {
		return &ApplyResourceLimiterOutput{Allowed: true}, nil
	}

	if resource == "" || group == "" {
		return &ApplyResourceLimiterOutput{Allowed: false, RetryAfterSecs: windowSec}, nil
	}

	now := in.NowUTC
	if now.IsZero() {
		now = time.Now().UTC()
	}

	windowID := now.Unix() / int64(windowSec)
	key := fmt.Sprintf("%s:%s:%d", group, resource, windowID)

	ttl := time.Duration(windowSec)*time.Second + time.Second
	newCount, err := l.redis.IncrementWithTTL(ctx, key, ttl)
	if err != nil {
		l.log.Error("ResourceLimiter.ApplyResourceLimiter increment failed",
			zap.String("key", key),
			zap.Error(err))
		return &ApplyResourceLimiterOutput{Allowed: false}, err
	}

	nextWindowStart := (windowID + 1) * int64(windowSec)
	retryAfter := int(nextWindowStart-now.Unix()) + 1

	if newCount > maxQuota {
		return &ApplyResourceLimiterOutput{Allowed: false, RetryAfterSecs: retryAfter}, nil
	}

	return &ApplyResourceLimiterOutput{Allowed: true}, nil
}
