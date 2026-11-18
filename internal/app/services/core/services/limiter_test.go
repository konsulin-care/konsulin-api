package services

import (
	"context"
	"testing"
	"time"

	"github.com/konsulin-care/konsulin-api/internal/app/contracts"
	"github.com/konsulin-care/konsulin-api/internal/pkg/constvars"

	"github.com/stretchr/testify/assert"
)

// fakeRedis is an in-memory contracts.RedisRepository double counting
// IncrementWithTTL calls per key.
type fakeRedis struct {
	counts map[string]int
}

var _ contracts.RedisRepository = (*fakeRedis)(nil)

func newFakeRedis() *fakeRedis {
	return &fakeRedis{counts: make(map[string]int)}
}

func (f *fakeRedis) Delete(context.Context, string) error { return nil }
func (f *fakeRedis) Set(context.Context, string, interface{}, time.Duration) error {
	return nil
}
func (f *fakeRedis) Get(context.Context, string) (string, error) { return "", nil }
func (f *fakeRedis) Increment(context.Context, string) error     { return nil }
func (f *fakeRedis) IncrementWithTTL(_ context.Context, key string, _ time.Duration) (int, error) {
	f.counts[key]++
	return f.counts[key], nil
}
func (f *fakeRedis) PushToList(context.Context, string, ...interface{}) error { return nil }
func (f *fakeRedis) PopFromList(context.Context, string) error                { return nil }
func (f *fakeRedis) AddToSet(context.Context, string, ...interface{}) error   { return nil }
func (f *fakeRedis) GetSetMembers(context.Context, string) ([]string, error)  { return nil, nil }
func (f *fakeRedis) TrySetNX(context.Context, string, interface{}, time.Duration) (bool, error) {
	return false, nil
}

// TestServiceLimiterRejectsWith429 covers assertion 2: once a user exceeds
// the per-listing limit inside the window the limiter answers Allowed=false
// with StatusCode 429, and other users or listings are unaffected.
func TestServiceLimiterRejectsWith429(t *testing.T) {
	lim := NewServiceLimiter(newFakeRedis())
	now := time.Unix(1_700_000_000, 0).UTC()

	// Three allowed calls, the fourth is rejected with 429.
	for i := 1; i <= 3; i++ {
		out, err := lim.Check(context.Background(), "usr-1", "interpret", 3, 60, now)
		assert.NoError(t, err)
		assert.True(t, out.Allowed, "call %d inside quota must pass", i)
		assert.Equal(t, constvars.StatusOK, out.StatusCode)
	}

	out, err := lim.Check(context.Background(), "usr-1", "interpret", 3, 60, now)
	assert.NoError(t, err)
	assert.False(t, out.Allowed, "the call past the per-listing limit must be rejected")
	assert.Equal(t, constvars.StatusTooManyRequests, out.StatusCode, "rejection must carry 429")
	assert.Positive(t, out.RetryAfterSecs, "a rejected call must report a retry delay")

	// Another user on the same listing keeps its own quota.
	out, err = lim.Check(context.Background(), "usr-2", "interpret", 3, 60, now)
	assert.NoError(t, err)
	assert.True(t, out.Allowed, "rate limiting is per user, not global")

	// The same user on another listing keeps its own quota too.
	out, err = lim.Check(context.Background(), "usr-1", "modify-profile", 3, 60, now)
	assert.NoError(t, err)
	assert.True(t, out.Allowed, "rate limiting is per listing, not per user")
}
