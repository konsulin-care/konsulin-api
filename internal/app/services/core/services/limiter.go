package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/pkg/constvars"
)

// LimiterOutput reports one rate-limit decision for a service invocation.
type LimiterOutput struct {
	// Allowed is false once the per-listing quota is exhausted in the window.
	Allowed bool
	// RetryAfterSecs is the wait until the next window (0 when allowed).
	RetryAfterSecs int
	// StatusCode is constvars.StatusTooManyRequests (429) when denied.
	StatusCode int
}

// ServiceLimiter enforces a Redis fixed-window limit keyed by user and
// listing: each user gets an independent quota per service listing inside
// a sliding-independent fixed window.
type ServiceLimiter struct {
	redis contracts.RedisRepository
}

// NewServiceLimiter builds a ServiceLimiter over the shared Redis repository.
//
// Parameters:
//   - redis provides the atomic increment-with-TTL primitive.
//
// Returns: a limiter ready to serve Check. Never panics.
func NewServiceLimiter(redis contracts.RedisRepository) *ServiceLimiter {
	return &ServiceLimiter{redis: redis}
}

// Check increments the (user, listing, window) counter and reports whether
// the request may proceed. Exceeding limit inside the current window yields
// Allowed=false with StatusCode 429 and a RetryAfterSecs countdown to the
// next window boundary. A non-positive limit disables the limiter (allow).
//
// Parameters:
//   - ctx bounds the Redis call.
//   - userID is the rate-limit subject ("" denies the request).
//   - listing is the service listing name ("" denies the request).
//   - limit is the max requests per window per listing.
//   - windowSec is the fixed window length (<=0 defaults to 60).
//   - now is the evaluation instant; zero means time.Now().UTC().
//
// Returns: the decision, or a Redis error. Never panics.
func (l *ServiceLimiter) Check(ctx context.Context, userID, listing string,
	limit, windowSec int, now time.Time,
) (*LimiterOutput, error) {
	if windowSec <= 0 {
		windowSec = 60
	}
	if limit <= 0 {
		return &LimiterOutput{Allowed: true, StatusCode: constvars.StatusOK}, nil
	}
	if l == nil || l.redis == nil {
		return &LimiterOutput{
			Allowed:        false,
			RetryAfterSecs: windowSec,
			StatusCode:     constvars.StatusTooManyRequests,
		}, nil
	}
	user := strings.TrimSpace(userID)
	name := strings.ToLower(strings.TrimSpace(listing))
	if user == "" || name == "" {
		return &LimiterOutput{
			Allowed:        false,
			RetryAfterSecs: windowSec,
			StatusCode:     constvars.StatusTooManyRequests,
		}, nil
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	windowID := now.Unix() / int64(windowSec)
	key := fmt.Sprintf("svc-limit:%s:%s:%d", name, user, windowID)
	ttl := time.Duration(windowSec)*time.Second + time.Second
	count, err := l.redis.IncrementWithTTL(ctx, key, ttl)
	if err != nil {
		return nil, err
	}
	if count > limit {
		nextStart := (windowID + 1) * int64(windowSec)
		return &LimiterOutput{
			Allowed:        false,
			RetryAfterSecs: int(nextStart-now.Unix()) + 1,
			StatusCode:     constvars.StatusTooManyRequests,
		}, nil
	}
	return &LimiterOutput{Allowed: true, StatusCode: constvars.StatusOK}, nil
}
