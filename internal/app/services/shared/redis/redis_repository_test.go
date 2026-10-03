package redis

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	redisclient "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// Intercept the SDK's command boundary so these tests assert the actual Redis
// commands and serialized values without requiring a shared cache server.
type commandHook struct{ run func(redisclient.Cmder) error }

func (commandHook) DialHook(next redisclient.DialHook) redisclient.DialHook { return next }
func (commandHook) ProcessPipelineHook(next redisclient.ProcessPipelineHook) redisclient.ProcessPipelineHook {
	return next
}

func (h commandHook) ProcessHook(_ redisclient.ProcessHook) redisclient.ProcessHook {
	return func(_ context.Context, cmd redisclient.Cmder) error { err := h.run(cmd); cmd.SetErr(err); return err }
}

func repositoryForTest(t *testing.T, run func(redisclient.Cmder) error) *redisRepository {
	t.Helper()
	client := redisclient.NewClient(&redisclient.Options{Addr: "unused.invalid:6379"})
	client.AddHook(commandHook{run: run})
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	return &redisRepository{Client: client, Log: zap.NewNop()}
}

// outcomes drives each command through a successful and a failing server reply.
var outcomes = []struct {
	suffix string
	err    error
}{{"/success", nil}, {"/failure", errors.New("cache failure")}}

// requireCacheError asserts that got carries want, or is nil when no failure is expected.
func requireCacheError(t *testing.T, want, got error) {
	t.Helper()
	if want == nil {
		require.NoError(t, got)
		return
	}
	require.ErrorContains(t, got, want.Error())
}

type commandCase struct {
	name    string
	command string
	args    []interface{}
	call    func(*redisRepository) error
}

func TestRedisCommandContracts(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []commandCase{
		{"delete", "del", []interface{}{"del", "key"}, func(r *redisRepository) error { return r.Delete(ctx, "key") }},
		{"increment", "incr", []interface{}{"incr", "key"}, func(r *redisRepository) error { return r.Increment(ctx, "key") }},
		{"push", "rpush", []interface{}{"rpush", "key", "a", "b"}, func(r *redisRepository) error { return r.PushToList(ctx, "key", "a", "b") }},
		{"pop", "lpop", []interface{}{"lpop", "key"}, func(r *redisRepository) error { return r.PopFromList(ctx, "key") }},
		{"set members", "sadd", []interface{}{"sadd", "key", "a", "b"}, func(r *redisRepository) error { return r.AddToSet(ctx, "key", "a", "b") }},
	} {
		for _, o := range outcomes {
			t.Run(tc.name+o.suffix, func(t *testing.T) { runCommandCase(t, tc, o.err) })
		}
	}
}

func runCommandCase(t *testing.T, tc commandCase, failure error) {
	calls := 0
	r := repositoryForTest(t, func(cmd redisclient.Cmder) error {
		calls++
		require.Equal(t, tc.command, cmd.Name())
		require.Equal(t, tc.args, cmd.Args())
		return failure
	})
	requireCacheError(t, failure, tc.call(r))
	require.Equal(t, 1, calls)
}

// writeCase describes a JSON write: the exact command it sends and the reply
// the server gives when the write succeeds.
type writeCase struct {
	name  string
	args  []interface{}
	reply func(redisclient.Cmder)
	write func(*redisRepository, interface{}) (bool, error)
}

func TestRedisSetAndSetNXSerializeJSONAndExpiration(t *testing.T) {
	ctx := context.Background()
	setArgs := []interface{}{"set", "key", []byte(`{"count":2}`), "px", int64(1500)}
	// Set has no acquired flag, so success stands in for it to share TrySetNX's contract.
	set := func(r *redisRepository, v interface{}) (bool, error) {
		err := r.Set(ctx, "key", v, 1500*time.Millisecond)
		return err == nil, err
	}
	trySetNX := func(r *redisRepository, v interface{}) (bool, error) {
		return r.TrySetNX(ctx, "key", v, 1500*time.Millisecond)
	}
	writes := []writeCase{
		{"set", setArgs, func(cmd redisclient.Cmder) { cmd.(*redisclient.StatusCmd).SetVal("OK") }, set},
		{"NX", append(slices.Clone(setArgs), "nx"), func(cmd redisclient.Cmder) { cmd.(*redisclient.BoolCmd).SetVal(true) }, trySetNX},
	}
	for _, tc := range writes {
		for _, o := range outcomes {
			t.Run(tc.name+o.suffix, func(t *testing.T) { runWriteCase(t, tc, o.err) })
		}
	}
	r := repositoryForTest(t, func(redisclient.Cmder) error { t.Fatal("marshal failures must not reach Redis"); return nil })
	for _, tc := range writes {
		acquired, err := tc.write(r, func() {})
		require.Error(t, err)
		require.False(t, acquired)
	}
}

func runWriteCase(t *testing.T, tc writeCase, failure error) {
	r := repositoryForTest(t, func(cmd redisclient.Cmder) error {
		require.Equal(t, tc.args, cmd.Args())
		if failure == nil {
			tc.reply(cmd)
		}
		return failure
	})
	acquired, err := tc.write(r, map[string]int{"count": 2})
	require.Equal(t, failure == nil, acquired)
	requireCacheError(t, failure, err)
}

func TestRedisGetAndSetMembersResults(t *testing.T) {
	failure := errors.New("read failure")
	for _, tc := range []struct {
		name  string
		value string
		err   error
	}{
		{"present", `"stored"`, nil}, {"missing", "", redisclient.Nil}, {"failure", "", failure},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := repositoryForTest(t, func(cmd redisclient.Cmder) error {
				require.Equal(t, []interface{}{"get", "key"}, cmd.Args())
				cmd.(*redisclient.StringCmd).SetVal(tc.value)
				return tc.err
			})
			value, err := r.Get(context.Background(), "key")
			require.Equal(t, tc.value, value)
			if tc.err == failure {
				require.ErrorContains(t, err, failure.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
	for _, fails := range []bool{false, true} {
		r := repositoryForTest(t, func(cmd redisclient.Cmder) error {
			require.Equal(t, []interface{}{"smembers", "key"}, cmd.Args())
			if fails {
				return failure
			}
			cmd.(*redisclient.StringSliceCmd).SetVal([]string{"a", "b"})
			return nil
		})
		members, err := r.GetSetMembers(context.Background(), "key")
		if fails {
			require.ErrorContains(t, err, failure.Error())
		} else {
			require.NoError(t, err)
			require.Equal(t, []string{"a", "b"}, members)
		}
	}
}

func TestRedisIncrementWithTTLResults(t *testing.T) {
	failure := errors.New("increment failure")
	for _, tc := range []struct {
		name    string
		result  interface{}
		err     error
		want    int
		wantErr bool
	}{
		{"first", int64(1), nil, 1, false}, {"subsequent", int64(7), nil, 7, false}, {"failure", nil, failure, 0, true}, {"unexpected server result", "invalid", nil, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := repositoryForTest(t, func(cmd redisclient.Cmder) error {
				require.Equal(t, "evalsha", cmd.Name())
				require.Equal(t, 1, cmd.Args()[2])
				require.Equal(t, "key", cmd.Args()[3])
				require.Equal(t, int64(1500), cmd.Args()[4])
				cmd.(*redisclient.Cmd).SetVal(tc.result)
				return tc.err
			})
			value, err := r.IncrementWithTTL(context.Background(), "key", 1500*time.Millisecond)
			require.Equal(t, tc.want, value)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			if tc.err != nil {
				require.ErrorContains(t, err, tc.err.Error())
			}
		})
	}
}

func TestNewRedisRepositorySingleton(t *testing.T) {
	client := redisclient.NewClient(&redisclient.Options{})
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	first := NewRedisRepository(client, zap.NewNop())
	require.Same(t, first, NewRedisRepository(nil, zap.NewNop()))
	require.Same(t, client, first.(*redisRepository).Client)
}
