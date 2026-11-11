package gateway

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/konsulin-care/gateway-sdk-go/wire"
	"github.com/redis/go-redis/v9"
)

// replayKeyPrefix namespaces replay reservations in Redis:
// gw:replay:<scope>:<key>.
const replayKeyPrefix = "gw:replay:"

// ReplayCache is the Redis-backed implementation of the SDK's
// wire.ReplayCache (BK-FED-006): Check reserves a (scope, key) pair with an
// atomic SET NX PX so concurrent callers and separate instances deduplicate
// against the same store. Any Redis failure returns an error — callers must
// deny (fail closed). Safe for concurrent use.
type ReplayCache struct {
	client *redis.Client
}

// Compile-time proof the adapter satisfies the SDK interface.
var _ wire.ReplayCache = (*ReplayCache)(nil)

// NewReplayCache builds a Redis-backed ReplayCache over a shared client.
//
// Parameters: client is the Redis client; a nil client yields a cache whose
// Check and Release always fail closed with an error.
// Returns: the cache, ready to plug into the SDK as wire.ReplayCache.
func NewReplayCache(client *redis.Client) *ReplayCache {
	return &ReplayCache{client: client}
}

// Check atomically reserves the (scope, key) pair until until via a single
// SET NX PX: the first Check of a fresh pair is accepted, a pair still
// reserved is a replay, and any Redis failure returns an error so callers
// deny (fail closed).
//
// Parameters: ctx carries caller cancellation; scope is a wire.Scope*
// constant; key is the scoped value; until is the reservation deadline.
// Returns: true when the pair was fresh, false on replay, or an error —
// deny on error. Never panics.
func (c *ReplayCache) Check(ctx context.Context, scope, key string, until time.Time) (bool, error) {
	if c == nil || c.client == nil {
		return false, errors.New("gateway: replay cache: no redis client")
	}
	ttl := time.Until(until)
	if ttl <= 0 {
		// The deadline already passed: nothing can be reserved, so the
		// pair stays unreserved and counts as fresh.
		return true, nil
	}
	set, err := c.client.SetNX(ctx, replayKeyPrefix+scope+":"+key, "1", ttl).Result()
	if err != nil {
		return false, fmt.Errorf("gateway: replay cache check: %w", err)
	}
	return set, nil
}

// Release frees the (scope, key) reservation so the pair is fresh again;
// unreserved pairs are a no-op.
//
// Parameters: ctx carries caller cancellation; scope and key as Check.
// Returns: nil on success, or an error when Redis fails — deny on error.
func (c *ReplayCache) Release(ctx context.Context, scope, key string) error {
	if c == nil || c.client == nil {
		return errors.New("gateway: replay cache: no redis client")
	}
	if err := c.client.Del(ctx, replayKeyPrefix+scope+":"+key).Err(); err != nil {
		return fmt.Errorf("gateway: replay cache release: %w", err)
	}
	return nil
}
