// Package federation implements the party enrollment lifecycle the gateway
// reports as the only source of truth (BK-PARTYID-002, BK-PARTYID-003):
// first-startup enrollment, bounded background retry and self-service key
// rotation, surfaced to delivery through contracts.GatewayStatus.
package federation

import (
	"context"
	"strings"
	"sync"
	"time"

	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/contracts"

	"go.uber.org/zap"
)

// Compile-time proof that Enroller satisfies contracts.GatewayStatus: the
// interface and the implementation must land together.
var _ contracts.GatewayStatus = (*Enroller)(nil)

// Bounded retry backoff: the first retry waits retryBackoffInitial, every
// following retry doubles the delay, capped at retryBackoffMax.
const (
	retryBackoffInitial = time.Second
	retryBackoffMax     = 30 * time.Second
)

// Clock creates the timers the background retry loop waits on. Tests inject
// a fake clock to drive backoff deterministically.
type Clock interface {
	// After returns a channel that receives once d has elapsed.
	After(d time.Duration) <-chan time.Time
}

// systemClock is the production Clock backed by time.After.
type systemClock struct{}

// After returns a channel that receives once d has elapsed (time.After).
func (systemClock) After(d time.Duration) <-chan time.Time { return time.After(d) }

// Enroller drives the party enrollment lifecycle: it probes the party JWKS
// (the gateway is the only enrollment source of truth), enrolls with the
// single-use APP_PARTY_ENROLLMENT_TOKEN when the own kid is absent, retries
// in the background with bounded backoff until ctx is done, and rotates to
// the APP_PARTY_NEXT_* keys once enrolled. It never logs key or token
// values. Implements contracts.GatewayStatus.
type Enroller struct {
	registry contracts.PartyRegistry
	token    string
	nextSig  string
	nextEnc  string
	enabled  bool
	log      *zap.Logger
	clock    Clock

	mu       sync.RWMutex
	enrolled bool
}

// NewEnroller builds the enrollment service over a party registry.
// Parameters: registry drives the JWKS probe, enroll and rotate calls; cfg
// supplies the gateway configuration — Enabled plus the sensitive
// APP_PARTY_ENROLLMENT_TOKEN and APP_PARTY_NEXT_* values, stored but never
// logged; log is the service logger (nil becomes a no-op logger); clock
// backs the retry backoff (nil becomes the system clock).
// Returns the enroller; never panics.
func NewEnroller(registry contracts.PartyRegistry, cfg config.AppGateway, log *zap.Logger, clock Clock) *Enroller {
	if log == nil {
		log = zap.NewNop()
	}
	if clock == nil {
		clock = systemClock{}
	}
	return &Enroller{
		registry: registry,
		token:    strings.TrimSpace(cfg.EnrollmentToken),
		nextSig:  strings.TrimSpace(cfg.NextPrivateKey),
		nextEnc:  strings.TrimSpace(cfg.NextEncPrivateKey),
		enabled:  cfg.Enabled,
		log:      log,
		clock:    clock,
	}
}

// Start performs the first enrollment pass synchronously — probe the party
// JWKS, enroll with the token when the own kid is absent — and, while the
// party stays un-enrolled, keeps retrying in the background with bounded
// backoff until ctx is done. Once enrolled it logs the reminder to remove
// APP_PARTY_ENROLLMENT_TOKEN (when one is set) and, when both
// APP_PARTY_NEXT_* keys are configured, rotates and logs the promote
// reminder.
// Parameters: ctx bounds every registry call and stops the retry loop.
// Returns nothing; the outcome is observable through Enrolled.
func (e *Enroller) Start(ctx context.Context) {
	if e.attempt(ctx) {
		return
	}
	go e.retry(ctx)
}

// attempt runs one probe-and-enroll pass and reports whether the party is
// enrolled afterwards: a JWKS already holding the own kid counts as enrolled
// without an enroll call, otherwise Enroll submits the token; any probe or
// enroll failure leaves the party un-enrolled.
// Parameters: ctx bounds the registry calls.
// Returns whether the pass ended with the party enrolled. Never panics.
func (e *Enroller) attempt(ctx context.Context) bool {
	enrolled, err := e.registry.IsEnrolled(ctx)
	if err != nil {
		e.log.Warn("federation: enrollment probe failed, will retry", zap.Error(err))
		return false
	}
	if !enrolled {
		if err := e.registry.Enroll(ctx, e.token); err != nil {
			e.log.Warn("federation: enroll failed, will retry", zap.Error(err))
			return false
		}
	}
	e.markEnrolled(ctx)
	return true
}

// markEnrolled records the enrolled status, logs the reminder to remove the
// single-use APP_PARTY_ENROLLMENT_TOKEN when one is configured, and — when
// both APP_PARTY_NEXT_* keys are set — performs one rotation and logs the
// reminder to promote the next keys. A rotation failure is logged only;
// enrollment itself stays in effect.
// Parameters: ctx bounds the rotation call.
// Returns nothing; never panics.
func (e *Enroller) markEnrolled(ctx context.Context) {
	e.mu.Lock()
	e.enrolled = true
	e.mu.Unlock()

	if e.token != "" {
		e.log.Info("federation: party enrolled, remove APP_PARTY_ENROLLMENT_TOKEN from the environment, it is single-use")
	}
	if e.nextSig == "" || e.nextEnc == "" {
		return
	}
	if err := e.registry.Rotate(ctx, e.nextSig, e.nextEnc); err != nil {
		e.log.Error("federation: key rotation failed", zap.Error(err))
		return
	}
	e.log.Info("federation: rotated to the next signing key, promote APP_PARTY_NEXT_PRIVATE_KEY and APP_PARTY_NEXT_ENC_PRIVATE_KEY to APP_PARTY_PRIVATE_KEY and APP_PARTY_ENC_PRIVATE_KEY, then unset the next keys")
}

// retry re-attempts enrollment in the background with bounded exponential
// backoff (retryBackoffInitial doubling up to retryBackoffMax) until the
// party is enrolled or ctx is done.
// Parameters: ctx stops the loop and bounds every registry call.
// Returns nothing; never panics.
func (e *Enroller) retry(ctx context.Context) {
	delay := retryBackoffInitial
	for {
		select {
		case <-ctx.Done():
			return
		case <-e.clock.After(delay):
		}
		if ctx.Err() != nil {
			return
		}
		if e.attempt(ctx) {
			return
		}
		delay = min(delay*2, retryBackoffMax)
	}
}

// Enabled reports whether the gateway integration is configured (a set
// APP_GATEWAY_URL). Implements contracts.GatewayStatus.
func (e *Enroller) Enabled() bool { return e.enabled }

// Enrolled reports the enrollment state last observed from the gateway:
// true once the party JWKS held the adapter's own kid or an enroll call
// succeeded, false until then. It performs no I/O. Implements
// contracts.GatewayStatus.
func (e *Enroller) Enrolled() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.enrolled
}
