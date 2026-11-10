package gateway

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/http"
	"testing"
	"time"

	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/contracts"

	"github.com/konsulin-care/gateway-sdk-go/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// countingTransport fails every RoundTrip and counts the attempts, so a test
// can prove the adapter performed no network I/O at all.
type countingTransport struct {
	hits *int
}

// RoundTrip records one attempted request and fails it immediately.
// Parameters: req is the outgoing request (never sent).
// Returns: nil and an error — no network I/O ever happens.
func (t countingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	*t.hits++
	return nil, errors.New("network I/O is forbidden in this test")
}

// TestNewEmptyURLDisabled covers assertion 1: gateway.New with an empty
// gateway URL returns an adapter whose Enabled() is false and whose calls
// return contracts.ErrGatewayNotConfigured without network I/O.
// Parameters: t is the test harness.
// Returns: nothing; fails on any deviation.
func TestNewEmptyURLDisabled(t *testing.T) {
	var hits int
	orig := http.DefaultTransport
	http.DefaultTransport = countingTransport{hits: &hits}
	defer func() { http.DefaultTransport = orig }()

	c, err := New(config.AppGateway{}, "test", nil)
	require.NoError(t, err)
	assert.False(t, c.Enabled(), "empty APP_GATEWAY_URL must yield a disabled adapter")
	assert.Nil(t, c.sdk, "a disabled adapter must hold no SDK client")

	// A cancelled context proves the short-circuit happens before any I/O:
	// an attempt would fail with context.Canceled, not ErrGatewayNotConfigured.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = c.Do(ctx, Request{Method: http.MethodGet, Path: "/health"})
	require.ErrorIs(t, err, contracts.ErrGatewayNotConfigured)

	assert.Equal(t, 0, hits, "disabled adapter must not touch the network")
}

// TestKidIsRFC7638Thumbprint covers assertion 2: the adapter's Kid() equals
// the RFC 7638 thumbprint the SDK derives for APP_PARTY_PRIVATE_KEY.
// Parameters: t is the test harness.
// Returns: nothing; fails on any deviation.
func TestKidIsRFC7638Thumbprint(t *testing.T) {
	seed := make([]byte, ed25519.SeedSize)
	for i := range seed {
		seed[i] = byte(i)
	}
	priv := ed25519.NewKeyFromSeed(seed)
	cfg := config.AppGateway{
		URL:        "https://gateway.example.com",
		PartyID:    "party-test-001",
		PrivateKey: base64.StdEncoding.EncodeToString(seed), // genkey output format
	}

	c, err := New(cfg, "production", nil)
	require.NoError(t, err)
	require.True(t, c.Enabled())

	// Expected value computed independently per RFC 7638 section 3.1:
	// SHA-256 over the canonical OKP JWK JSON, base64url without padding.
	pub := priv.Public().(ed25519.PublicKey)
	canonical := `{"crv":"Ed25519","kty":"OKP","x":"` +
		base64.RawURLEncoding.EncodeToString(pub) + `"}`
	sum := sha256.Sum256([]byte(canonical))
	want := base64.RawURLEncoding.EncodeToString(sum[:])

	// The SDK must derive the very same thumbprint for its ks-party+jwt kid.
	iat := time.Now().Unix()
	tok, err := auth.SignPartyRequest(priv, auth.PartyClaims{
		Aud: "gateway",
		Exp: iat + auth.MaxPartyLifetime,
		Iat: iat,
		Iss: cfg.PartyID,
		Jti: "thumbprint-check",
	})
	require.NoError(t, err)
	hdr, err := decodeHeader(tok)
	require.NoError(t, err)
	require.NotEmpty(t, hdr.Kid)
	assert.Equal(t, want, hdr.Kid, "SDK kid must be the RFC 7638 thumbprint")

	assert.Equal(t, want, c.Kid(), "adapter Kid() must equal the SDK RFC 7638 thumbprint")
}
