package gateway

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/konsulin-care/gateway-sdk-go/wire"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestReplayCache spins a miniredis server and a ReplayCache over it.
// The server is closed automatically when the test finishes.
func newTestReplayCache(t *testing.T) (*miniredis.Miniredis, *ReplayCache) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	return mr, NewReplayCache(client)
}

// TestReplayCacheFirstCheckAccepted covers assertion 1: the first Check for
// a scope/key pair returns true and the reservation lives under the
// gw:replay:<scope>:<key> key.
// Parameters: t is the test harness.
// Returns: nothing; fails on any deviation.
func TestReplayCacheFirstCheckAccepted(t *testing.T) {
	mr, cache := newTestReplayCache(t)

	ok, err := cache.Check(context.Background(), wire.ScopeJTI, "token-1", time.Now().Add(time.Minute))
	require.NoError(t, err)
	assert.True(t, ok, "the first Check for a scope/key pair must be accepted")
	assert.True(t, mr.Exists("gw:replay:jti:token-1"),
		"the reservation must live under the gw:replay:<scope>:<key> prefix")
}

// TestReplayCacheRepeatIsReplay covers assertion 2: a repeated Check for
// the same scope/key before expiry returns false.
// Parameters: t is the test harness.
// Returns: nothing; fails on any deviation.
func TestReplayCacheRepeatIsReplay(t *testing.T) {
	_, cache := newTestReplayCache(t)
	ctx := context.Background()
	until := time.Now().Add(time.Minute)

	first, err := cache.Check(ctx, wire.ScopeJTI, "token-1", until)
	require.NoError(t, err)
	require.True(t, first, "the first Check must be accepted")

	again, err := cache.Check(ctx, wire.ScopeJTI, "token-1", until)
	require.NoError(t, err)
	assert.False(t, again, "a repeated Check before expiry must be reported as a replay")

	// A different scope with the same key stays fresh.
	other, err := cache.Check(ctx, wire.ScopeNonce, "token-1", until)
	require.NoError(t, err)
	assert.True(t, other, "scopes must keep separate key spaces")
}

// TestReplayCacheRedisFailureDenies covers assertion 3: a Redis failure
// makes Check return an error so callers deny (fail closed). Both failure
// modes are exercised: a server closed mid-test and a client pointed at a
// dead address.
// Parameters: t is the test harness.
// Returns: nothing; fails on any deviation.
func TestReplayCacheRedisFailureDenies(t *testing.T) {
	t.Run("closed server", func(t *testing.T) {
		mr, cache := newTestReplayCache(t)
		mr.Close()

		ok, err := cache.Check(context.Background(), wire.ScopeJTI, "token-1", time.Now().Add(time.Minute))
		require.Error(t, err, "a Redis failure must surface as an error so callers deny")
		assert.False(t, ok, "a failed Check must not accept the pair")
	})

	t.Run("dead address", func(t *testing.T) {
		dead := NewReplayCache(redis.NewClient(&redis.Options{
			Addr:        "127.0.0.1:1",
			DialTimeout: 250 * time.Millisecond,
			MaxRetries:  0,
		}))

		ok, err := dead.Check(context.Background(), wire.ScopeJTI, "token-1", time.Now().Add(time.Minute))
		require.Error(t, err, "a Redis failure must surface as an error so callers deny")
		assert.False(t, ok, "a failed Check must not accept the pair")
	})
}
