package gateway

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"net/http"
	"strings"
	"testing"

	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/contracts"

	"github.com/konsulin-care/gateway-sdk-go/gatewaytest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// partySeed builds the deterministic test party seed: seed byte i equals
// fill+i, so distinct fills yield distinct Ed25519 keys.
// Parameters: fill marks the first seed byte.
// Returns: the 32-byte Ed25519 seed.
func partySeed(fill byte) []byte {
	seed := make([]byte, ed25519.SeedSize)
	for i := range seed {
		seed[i] = byte(int(fill) + i)
	}
	return seed
}

// partyKey derives the main test party key the adapter under test signs with.
// Parameters: t is the test harness.
// Returns: the Ed25519 private key matching newIsEnrolledAdapter's config.
func partyKey(t *testing.T) ed25519.PrivateKey {
	t.Helper()
	return ed25519.NewKeyFromSeed(partySeed(0))
}

// newIsEnrolledAdapter builds an enabled adapter against rawURL under env
// "test" (its loopback http:// URL requires WithInsecureTransport) using the
// deterministic main test party seed.
// Parameters: t is the test harness, rawURL the fake gateway base URL.
// Returns: the adapter; fails the test on error.
func newIsEnrolledAdapter(t *testing.T, rawURL string) *Client {
	t.Helper()
	cfg := config.AppGateway{
		URL:        rawURL,
		PartyID:    "party-test-001",
		PrivateKey: base64.StdEncoding.EncodeToString(partySeed(0)),
	}
	c, err := New(cfg, "test", nil)
	require.NoError(t, err)
	require.True(t, c.Enabled())
	require.NotEmpty(t, c.Kid())
	return c
}

// TestIsEnrolledTrueWhenPartyJWKSServesOwnKid covers the contract assertion:
// IsEnrolled returns true only when the party JWKS contains the adapter's own
// kid — here a fake JWKS endpoint serves a document built from the adapter's
// own key (kid = RFC 7638 thumbprint, the same value Kid() derives).
// Parameters: t is the test harness.
// Returns: nothing; fails on any deviation.
func TestIsEnrolledTrueWhenPartyJWKSServesOwnKid(t *testing.T) {
	jwks, err := gatewaytest.Ed25519JWKS(partyKey(t).Public().(ed25519.PublicKey))
	require.NoError(t, err)
	srv, err := gatewaytest.NewJWKSServer(jwks)
	require.NoError(t, err)
	t.Cleanup(srv.Close)

	c := newIsEnrolledAdapter(t, srv.URL())
	enrolled, err := c.IsEnrolled(context.Background())
	require.NoError(t, err, "a served JWKS with our kid must not error")
	assert.True(t, enrolled,
		"IsEnrolled must be true when the party JWKS contains the adapter's own kid")
}

// TestIsEnrolledFalseWhenPartyJWKSLacksOwnKid covers the other half of the
// contract: a JWKS that does not contain the adapter's own kid must report
// false (no error), whether the fake serves its canned kid or a foreign
// key's document.
// Parameters: t is the test harness.
// Returns: nothing; fails on any deviation.
func TestIsEnrolledFalseWhenPartyJWKSLacksOwnKid(t *testing.T) {
	t.Run("canned gateway serves only the fake kid", func(t *testing.T) {
		gw := gatewaytest.NewGateway()
		t.Cleanup(gw.Close)
		c := newIsEnrolledAdapter(t, gw.URL())

		enrolled, err := c.IsEnrolled(context.Background())
		require.NoError(t, err, "a JWKS without our kid is an answer, not an error")
		assert.False(t, enrolled,
			"IsEnrolled must be false when the party JWKS lacks the adapter's own kid")

		// The probe went through the SDK client: exactly one recorded GET on
		// the party JWKS path, party-signed with the adapter's own kid.
		recs := gw.Recorded()
		require.Len(t, recs, 1, "one Recorded entry per Do")
		require.Equal(t, http.MethodGet, recs[0].Method)
		require.Equal(t, "/v1/parties/party-test-001/jwks", recs[0].Path)
		authz := recs[0].Authorization
		require.True(t, strings.HasPrefix(authz, "Bearer "), "probe must carry a bearer ks-party+jwt")
		hdr, err := decodeHeader(strings.TrimSpace(strings.TrimPrefix(authz, "Bearer ")))
		require.NoError(t, err)
		assert.Equal(t, c.Kid(), hdr.Kid, "probe must be signed with the adapter kid")
	})

	t.Run("party jwks of a foreign key", func(t *testing.T) {
		foreign := ed25519.NewKeyFromSeed(partySeed(200))
		jwks, err := gatewaytest.Ed25519JWKS(foreign.Public().(ed25519.PublicKey))
		require.NoError(t, err)
		srv, err := gatewaytest.NewJWKSServer(jwks)
		require.NoError(t, err)
		t.Cleanup(srv.Close)

		c := newIsEnrolledAdapter(t, srv.URL())
		enrolled, err := c.IsEnrolled(context.Background())
		require.NoError(t, err)
		assert.False(t, enrolled,
			"a JWKS serving only a foreign kid must report false")
	})
}

// TestIsEnrolledDisabledAdapterFailsClosed proves a disabled adapter reports
// false with contracts.ErrGatewayNotConfigured without any network I/O.
// Parameters: t is the test harness.
// Returns: nothing; fails on any deviation.
func TestIsEnrolledDisabledAdapterFailsClosed(t *testing.T) {
	var hits int
	orig := http.DefaultTransport
	http.DefaultTransport = countingTransport{hits: &hits}
	defer func() { http.DefaultTransport = orig }()

	c, err := New(config.AppGateway{}, "test", nil)
	require.NoError(t, err)
	require.False(t, c.Enabled())

	enrolled, err := c.IsEnrolled(context.Background())
	assert.False(t, enrolled, "a disabled adapter must never report enrolled")
	require.ErrorIs(t, err, contracts.ErrGatewayNotConfigured)
	assert.Equal(t, 0, hits, "disabled adapter must not touch the network")
}

// TestIsEnrolledTransportErrorFailsClosed proves a failed JWKS probe reports
// false with the transport error instead of a false "enrolled" answer.
// Parameters: t is the test harness.
// Returns: nothing; fails on any deviation.
func TestIsEnrolledTransportErrorFailsClosed(t *testing.T) {
	jwks, err := gatewaytest.Ed25519JWKS(partyKey(t).Public().(ed25519.PublicKey))
	require.NoError(t, err)
	srv, err := gatewaytest.NewJWKSServer(jwks)
	require.NoError(t, err)
	t.Cleanup(srv.Close)
	c := newIsEnrolledAdapter(t, srv.URL())

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	enrolled, err := c.IsEnrolled(ctx)
	assert.False(t, enrolled, "a failed probe must fail closed")
	require.Error(t, err, "a failed probe must surface the transport error")
}
