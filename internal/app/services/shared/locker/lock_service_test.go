package locker

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"konsulin-service/internal/app/contracts"
)

type lockRedis struct {
	contracts.RedisRepository
	value            string
	getErr, writeErr error
	acquired         bool
	writes           int
	key              string
	ttl              time.Duration
}

func (r *lockRedis) Get(_ context.Context, key string) (string, error) {
	r.key = key
	return r.value, r.getErr
}

func (r *lockRedis) Delete(_ context.Context, key string) error {
	r.writes++
	r.key = key
	return r.writeErr
}

func (r *lockRedis) Set(_ context.Context, key string, value interface{}, ttl time.Duration) error {
	r.writes++
	r.key = key
	r.value = value.(string)
	r.ttl = ttl
	return r.writeErr
}

func (r *lockRedis) TrySetNX(ctx context.Context, key string, value interface{}, ttl time.Duration) (bool, error) {
	return r.acquired, r.Set(ctx, key, value, ttl)
}

func TestTryLockOwnershipToken(t *testing.T) {
	failure := errors.New("redis unavailable")
	for _, tc := range []struct {
		name     string
		acquired bool
		err      error
	}{
		{"acquired", true, nil}, {"held by another caller", false, nil}, {"write failure", false, failure},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &lockRedis{acquired: tc.acquired, writeErr: tc.err}
			service := &lockService{redisRepo: repo, Log: zap.NewNop()}
			acquired, token, err := service.TryLock(context.Background(), "job", time.Minute)
			require.ErrorIs(t, err, tc.err)
			require.Equal(t, tc.acquired, acquired)
			require.Equal(t, "job", repo.key)
			require.Equal(t, time.Minute, repo.ttl)
			if acquired {
				require.Equal(t, repo.value, token)
				_, err := uuid.Parse(token)
				require.NoError(t, err)
			} else {
				require.Empty(t, token)
			}
		})
	}
}

// lockOperation invokes Unlock or Refresh as the "owner" of the "job" lock.
type lockOperation struct {
	name      string
	call      func(*lockService) error
	refreshes bool // Refresh rewrites the value with a new TTL instead of deleting it
}

type ownershipCase struct {
	name, value      string
	getErr, writeErr error
	writes           int
	wantErr          bool
}

func runOwnershipCase(t *testing.T, op lockOperation, tc ownershipCase) {
	t.Helper()
	repo := &lockRedis{value: tc.value, getErr: tc.getErr, writeErr: tc.writeErr}
	err := op.call(&lockService{redisRepo: repo, Log: zap.NewNop()})
	require.Equal(t, tc.wantErr, err != nil, "unexpected error result: %v", err)
	if tc.getErr != nil {
		require.ErrorIs(t, err, tc.getErr)
	}
	if tc.writeErr != nil {
		require.ErrorIs(t, err, tc.writeErr)
	}
	require.Equal(t, tc.writes, repo.writes)
	require.Equal(t, "job", repo.key)
	if op.refreshes && tc.writes == 1 {
		require.Equal(t, time.Hour, repo.ttl)
		require.Equal(t, "owner", repo.value)
	}
}

func TestUnlockAndRefreshRespectOwnershipAndErrors(t *testing.T) {
	failure := errors.New("redis unavailable")
	operations := []lockOperation{
		{"unlock", func(s *lockService) error { return s.Unlock(context.Background(), "job", "owner") }, false},
		{"refresh", func(s *lockService) error { return s.Refresh(context.Background(), "job", "owner", time.Hour) }, true},
	}
	cases := []ownershipCase{
		{"missing", "", nil, nil, 0, false},
		{"owned", `"owner"`, nil, nil, 1, false},
		{"different owner", `"another"`, nil, nil, 0, true},
		{"read failure", "", failure, nil, 0, true},
		{"write failure", `"owner"`, nil, failure, 1, true},
	}
	for _, op := range operations {
		for _, tc := range cases {
			t.Run(op.name+"/"+tc.name, func(t *testing.T) { runOwnershipCase(t, op, tc) })
		}
	}
}

func TestNewLockServiceReturnsSingleton(t *testing.T) {
	repo := &lockRedis{}
	first := NewLockService(repo, zap.NewNop())
	require.Same(t, first, NewLockService(&lockRedis{}, zap.NewNop()))
	require.Same(t, repo, first.(*lockService).redisRepo)
}
