package main

import (
	"crypto/ecdh"
	"crypto/ed25519"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/konsulin-care/konsulin-api/internal/app/config"
	"github.com/konsulin-care/konsulin-api/internal/app/delivery/http/routers"
	"github.com/konsulin-care/konsulin-api/internal/pkg/constvars"

	"github.com/konsulin-care/gateway-sdk-go/gatewaytest"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// countingRoundTripper counts every call issued through the default
// transport so tests can prove a wiring path dials nothing.
type countingRoundTripper struct {
	hits *atomic.Int64
}

// errCountedDial fails any request the counting transport observes.
var errCountedDial = &countedDialError{}

// countedDialError marks a transport call the test counted and denied.
type countedDialError struct{}

// Error describes the denied dial.
// Parameters: none.
// Returns the fixed message. Never panics.
func (*countedDialError) Error() string { return "federation test: network call denied" }

// RoundTrip records one attempted call and fails it: the counting transport
// never reaches a real network.
// Parameters: req is the outbound request.
// Returns a non-nil error after counting the attempt. Never panics.
func (c countingRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	c.hits.Add(1)
	return nil, errCountedDial
}

// fedSeed builds a deterministic 32-byte seed: byte i equals fill+i, the
// same convention the adapter tests use.
// Parameters: fill marks the first seed byte.
// Returns the Ed25519/X25519 seed bytes.
func fedSeed(fill byte) []byte {
	seed := make([]byte, ed25519.SeedSize)
	for i := range seed {
		seed[i] = byte(int(fill) + i)
	}
	return seed
}

// fedBootstrap builds a minimal bootstrap carrying only what the federation
// wiring reads: the internal config and a no-op logger.
// Parameters: gatewayCfg is the gateway/party env configuration.
// Returns the bootstrap ready for newGateway/registerFederation.
func fedBootstrap(gatewayCfg config.AppGateway) *config.Bootstrap {
	return &config.Bootstrap{
		Logger: zap.NewNop(),
		InternalConfig: &config.InternalConfig{
			App:     config.App{Env: constvars.EnvTest},
			Gateway: gatewayCfg,
		},
	}
}

// TestRegisterFederationDisabledWhenGatewayURLEmpty covers the contract
// assertion: with an empty gateway URL the wiring yields a disabled status
// and performs no network calls. The adapter reports disabled, the status
// published on ctrls.Gateway reports disabled and never enrolled, and
// neither the default transport nor a live server squatting on
// APP_GATEWAY_URL sees a single call.
// Parameters: t is the test harness.
// Returns nothing; fails on any deviation.
func TestRegisterFederationDisabledWhenGatewayURLEmpty(t *testing.T) {
	var hits atomic.Int64
	orig := http.DefaultTransport
	http.DefaultTransport = countingRoundTripper{hits: &hits}
	defer func() { http.DefaultTransport = orig }()

	// Tripwire: even with a live gateway URL in the environment, an empty
	// gateway config must never dial it.
	envSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(envSrv.Close)
	t.Setenv("APP_GATEWAY_URL", envSrv.URL)

	bootstrap := fedBootstrap(config.AppGateway{})
	wiring, err := newGateway(bootstrap)
	require.NoError(t, err, "empty gateway URL must wire without error")
	require.False(t, wiring.client.Enabled(), "empty gateway URL must build a disabled adapter")
	require.NotNil(t, wiring.replay, "the Redis replay cache must be built")

	ctrls := &routers.Controllers{}
	registerFederation(bootstrap, ctrls, bootstrap.InternalConfig.Gateway, nil)

	require.NotNil(t, ctrls.Gateway, "the gateway status must be published on ctrls")
	require.False(t, ctrls.Gateway.Enabled(), "an empty gateway URL must yield a disabled status")
	require.False(t, ctrls.Gateway.Enrolled(), "a disabled status must never report enrolled")

	// Grace period: a wrongly started background enrollment would dial now.
	time.Sleep(100 * time.Millisecond)
	require.Zero(t, hits.Load(),
		"an empty gateway URL must perform no network calls")
}

// TestRegisterFederationEnrollsInBackgroundAgainstGatewaytestFake covers the
// contract assertion: with a gateway config pointing at a gatewaytest fake
// the background enrollment completes and the status reports enrolled.
// Parameters: t is the test harness.
// Returns nothing; fails on any deviation.
func TestRegisterFederationEnrollsInBackgroundAgainstGatewaytestFake(t *testing.T) {
	gw := gatewaytest.NewGateway()
	t.Cleanup(gw.Close)

	// Party key material: the adapter derives its kid from
	// APP_PARTY_PRIVATE_KEY and Enroll reads the APP_PARTY_* env values.
	partySeed := fedSeed(0)
	encSeed := fedSeed(100)
	_, err := ecdh.X25519().NewPrivateKey(encSeed)
	require.NoError(t, err, "the enc seed must parse as X25519")
	t.Setenv("APP_GATEWAY_URL", gw.URL())
	t.Setenv("APP_PARTY_PRIVATE_KEY", base64.StdEncoding.EncodeToString(partySeed))
	t.Setenv("APP_PARTY_ENC_PRIVATE_KEY", base64.StdEncoding.EncodeToString(encSeed))

	gatewayCfg := config.AppGateway{
		Enabled:         true,
		URL:             gw.URL(),
		PartyID:         "party-federation-001",
		PrivateKey:      base64.StdEncoding.EncodeToString(partySeed),
		EncPrivateKey:   base64.StdEncoding.EncodeToString(encSeed),
		EnrollmentToken: "enroll-token-federation-0001",
	}
	bootstrap := fedBootstrap(gatewayCfg)
	wiring, err := newGateway(bootstrap)
	require.NoError(t, err, "a gateway URL must wire without error")
	require.True(t, wiring.client.Enabled(), "a configured gateway URL must build an enabled adapter")

	ctrls := &routers.Controllers{}
	registerFederation(bootstrap, ctrls, bootstrap.InternalConfig.Gateway, nil)

	require.NotNil(t, ctrls.Gateway, "the gateway status must be published on ctrls")
	require.True(t, ctrls.Gateway.Enabled(), "a configured gateway must yield an enabled status")
	require.Eventually(t, ctrls.Gateway.Enrolled, 5*time.Second, 25*time.Millisecond,
		"the background enrollment must complete against the gatewaytest fake and the status reports enrolled")
}
