package federation

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"konsulin-service/internal/app/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

// kidFor derives a deterministic fake kid from key material, mirroring the
// RFC 7638 thumbprint the adapter derives from a signing key.
func kidFor(key string) string {
	sum := sha256.Sum256([]byte(key))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// fakeParty is a contracts.PartyRegistry fake modelling the gateway party:
// the JWKS serves servedKid, a successful Enroll puts the adapter's own kid
// there, and Rotate switches both the adapter and the served JWKS to the
// next key's kid.
type fakeParty struct {
	mu             sync.Mutex
	ownKid         string
	servedKid      string
	enrollFailures int
	enrollCalls    int
	rotateCalls    int
	rotatedSig     string
	rotatedEnc     string
}

// newFakeParty builds the fake for the adapter key; servedOwn reports
// whether the party JWKS already holds the adapter's own kid.
func newFakeParty(key string, servedOwn bool) *fakeParty {
	f := &fakeParty{ownKid: kidFor(key)}
	if servedOwn {
		f.servedKid = f.ownKid
	}
	return f
}

// failEnroll makes the next n Enroll calls fail before changing any state.
func (f *fakeParty) failEnroll(n int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.enrollFailures = n
}

// enrollCount returns how many Enroll calls reached the fake.
func (f *fakeParty) enrollCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.enrollCalls
}

// rotateCount returns how many Rotate calls reached the fake.
func (f *fakeParty) rotateCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.rotateCalls
}

// rotatedKeys returns the key values the last Rotate received.
func (f *fakeParty) rotatedKeys() (string, string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.rotatedSig, f.rotatedEnc
}

// currentKid returns the kid the party JWKS reports for the adapter.
func (f *fakeParty) currentKid() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.ownKid
}

// IsEnrolled reports whether the served JWKS holds the adapter's own kid.
func (f *fakeParty) IsEnrolled(context.Context) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.servedKid != "" && f.servedKid == f.ownKid, nil
}

// Enroll accepts the token and publishes the own kid in the JWKS, unless a
// queued failure rejects the call first.
func (f *fakeParty) Enroll(context.Context, string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.enrollCalls++
	if f.enrollFailures > 0 {
		f.enrollFailures--
		return errors.New("gateway: enroll: gateway rejected the enrollment")
	}
	f.servedKid = f.ownKid
	return nil
}

// Rotate publishes the next kid and switches the adapter signer to it, the
// way the real registry does after an accepted rotation.
func (f *fakeParty) Rotate(_ context.Context, nextSig, nextEnc string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rotateCalls++
	f.rotatedSig = nextSig
	f.rotatedEnc = nextEnc
	f.ownKid = kidFor(nextSig)
	f.servedKid = f.ownKid
	return nil
}

// fakeClock is a manual Clock: every After parks a waiter the test releases
// through tick, and each requested delay is recorded for assertions.
type fakeClock struct {
	mu      sync.Mutex
	waiters []chan time.Time
	delays  []time.Duration
}

// After parks the caller until tick releases the oldest waiter.
func (c *fakeClock) After(d time.Duration) <-chan time.Time {
	ch := make(chan time.Time, 1)
	c.mu.Lock()
	c.waiters = append(c.waiters, ch)
	c.delays = append(c.delays, d)
	c.mu.Unlock()
	return ch
}

// hasWaiter reports whether a retry is parked on the clock.
func (c *fakeClock) hasWaiter() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.waiters) > 0
}

// tick releases the oldest parked waiter, advancing the fake time.
func (c *fakeClock) tick() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.waiters) == 0 {
		return
	}
	ch := c.waiters[0]
	c.waiters = c.waiters[1:]
	ch <- time.Now()
}

// recorded returns the delays requested through After, in order.
func (c *fakeClock) recorded() []time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]time.Duration(nil), c.delays...)
}

// waitFor polls cond until it holds, failing the test after five seconds.
func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("condition not met within 5s")
}

// joinedLogs flattens the observed log messages into one assertable string.
func joinedLogs(logs *observer.ObservedLogs) string {
	entries := logs.All()
	msgs := make([]string, 0, len(entries))
	for _, entry := range entries {
		msgs = append(msgs, entry.Message)
	}
	return strings.Join(msgs, "\n")
}

// TestEnrollerStart_AlreadyEnrolled covers the contract line "when the party
// JWKS already holds the own kid the status becomes enrolled without an
// enroll call".
func TestEnrollerStart_AlreadyEnrolled(t *testing.T) {
	reg := newFakeParty("own-signing-key", true)
	core, logs := observer.New(zapcore.DebugLevel)
	e := NewEnroller(reg, config.AppGateway{
		Enabled:         true,
		EnrollmentToken: "single-use-token",
	}, zap.New(core), &fakeClock{})

	e.Start(context.Background())

	require.True(t, e.Enabled(), "gateway configured reports enabled")
	require.True(t, e.Enrolled(), "status must become enrolled")
	assert.Zero(t, reg.enrollCount(), "no enroll call when the JWKS holds the own kid")
	assert.Zero(t, reg.rotateCount(), "no rotation without APP_PARTY_NEXT_* keys")
	assert.Contains(t, joinedLogs(logs), "remove APP_PARTY_ENROLLMENT_TOKEN",
		"reminder logged when APP_PARTY_ENROLLMENT_TOKEN is set")
}

// TestEnrollerStart_FailedEnrollRetriesInBackground covers the contract line
// "a failed enrollment leaves the status not enrolled and a background retry
// later enrolls the party", including the bounded backoff sequence.
func TestEnrollerStart_FailedEnrollRetriesInBackground(t *testing.T) {
	reg := newFakeParty("own-signing-key", false)
	reg.failEnroll(6)
	clk := &fakeClock{}
	e := NewEnroller(reg, config.AppGateway{
		Enabled:         true,
		EnrollmentToken: "single-use-token",
	}, zap.NewNop(), clk)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	e.Start(ctx)

	require.False(t, e.Enrolled(), "failed enrollment must leave the status not enrolled")
	require.Equal(t, 1, reg.enrollCount(), "the first attempt runs synchronously inside Start")

	// Drive the background retry: each tick releases one bounded backoff wait.
	for i := 0; !e.Enrolled() && i < 20; i++ {
		waitFor(t, func() bool { return clk.hasWaiter() || e.Enrolled() })
		if e.Enrolled() {
			break
		}
		clk.tick()
	}
	waitFor(t, e.Enrolled)

	assert.Equal(t, 7, reg.enrollCount(), "six failed retries plus the successful attempt")
	wantDelays := []time.Duration{
		retryBackoffInitial,
		retryBackoffInitial * 2,
		retryBackoffInitial * 4,
		retryBackoffInitial * 8,
		retryBackoffInitial * 16,
		retryBackoffMax, // capped: doubling further would exceed the bound
	}
	assert.Equal(t, wantDelays, clk.recorded(), "backoff doubles and stays bounded at retryBackoffMax")
}

// TestEnrollerStart_NextKeysRotate covers the contract line "with next keys
// configured an enrolled party rotates and reports the next kid".
func TestEnrollerStart_NextKeysRotate(t *testing.T) {
	reg := newFakeParty("own-signing-key", true)
	const nextSig, nextEnc = "next-signing-key", "next-enc-key"
	core, logs := observer.New(zapcore.DebugLevel)
	e := NewEnroller(reg, config.AppGateway{
		Enabled:           true,
		EnrollmentToken:   "single-use-token",
		NextPrivateKey:    nextSig,
		NextEncPrivateKey: nextEnc,
	}, zap.New(core), &fakeClock{})

	e.Start(context.Background())

	require.True(t, e.Enrolled())
	assert.Zero(t, reg.enrollCount(), "already-enrolled party skips enrollment")
	require.Equal(t, 1, reg.rotateCount(), "enrolled party with next keys rotates once")
	gotSig, gotEnc := reg.rotatedKeys()
	assert.Equal(t, nextSig, gotSig)
	assert.Equal(t, nextEnc, gotEnc)
	assert.Equal(t, kidFor(nextSig), reg.currentKid(), "the party reports the next kid")
	assert.Contains(t, joinedLogs(logs), "APP_PARTY_NEXT_PRIVATE_KEY",
		"promote reminder logged after rotation")
}
