package gateway

import (
	"context"
	"crypto/ecdh"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/konsulin-care/konsulin-api/internal/app/config"
	"github.com/konsulin-care/konsulin-api/internal/app/contracts"

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

// TestEnrollSubmitsTokenKeysThenPartyEnrolled covers the T12B contract
// assertion: Enroll submits the enrollment token, the signing public key
// and enc_public_key (derived from APP_PARTY_ENC_PRIVATE_KEY) to the fake
// gateway's enroll route, and afterwards the enrollment probe reports the
// party enrolled on that fake.
// Parameters: t is the test harness.
// Returns: nothing; fails on any deviation.
func TestEnrollSubmitsTokenKeysThenPartyEnrolled(t *testing.T) {
	gw := gatewaytest.NewGateway()
	t.Cleanup(gw.Close)

	// Deterministic X25519 key: APP_PARTY_ENC_PRIVATE_KEY in the SDK's
	// registration encoding (std base64, raw 32 bytes).
	encSeed := partySeed(100)
	encPriv, err := ecdh.X25519().NewPrivateKey(encSeed)
	require.NoError(t, err)
	wantEnc := base64.StdEncoding.EncodeToString(encPriv.PublicKey().Bytes())

	rawURL := gw.URL()
	t.Setenv("APP_GATEWAY_URL", rawURL)
	t.Setenv("APP_PARTY_PRIVATE_KEY", base64.StdEncoding.EncodeToString(partySeed(0)))
	t.Setenv("APP_PARTY_ENC_PRIVATE_KEY", base64.StdEncoding.EncodeToString(encSeed))

	c := newIsEnrolledAdapter(t, rawURL)

	// Before enrollment the probe must report not-enrolled (canned JWKS
	// lacks our kid).
	enrolled, err := c.IsEnrolled(context.Background())
	require.NoError(t, err)
	assert.False(t, enrolled, "party must not be enrolled before Enroll")

	const token = "enroll-token-test-0001"
	require.NoError(t, c.Enroll(context.Background(), token),
		"gatewaytest fake must accept the enrollment")

	recs := gw.Recorded()
	require.Len(t, recs, 2, "one probe plus the one enroll request")
	enroll := recs[1]
	require.Equal(t, http.MethodPost, enroll.Method)
	require.Equal(t, "/v1/parties/party-test-001/enroll", enroll.Path)
	require.NotEmpty(t, enroll.IdempotencyKey, "enroll is an idempotent create")

	var body struct {
		EnrollmentToken string `json:"enrollment_token"`
		PublicKey       string `json:"public_key"`
		Kid             string `json:"kid"`
		Alg             string `json:"alg"`
		EncPublicKey    string `json:"enc_public_key"`
	}
	require.NoError(t, json.Unmarshal(enroll.Body, &body))
	assert.Equal(t, token, body.EnrollmentToken, "must submit the enrollment token")
	assert.Equal(t, base64.StdEncoding.EncodeToString(partyKey(t).Public().(ed25519.PublicKey)),
		body.PublicKey, "must submit the signing public key")
	assert.Equal(t, c.Kid(), body.Kid, "kid must be the adapter RFC 7638 thumbprint")
	assert.Equal(t, "Ed25519", body.Alg)
	assert.Equal(t, wantEnc, body.EncPublicKey,
		"must submit enc_public_key derived from APP_PARTY_ENC_PRIVATE_KEY")

	// The proof of possession is a bearer ks-party+jwt signed with the
	// adapter kid, verified offline against the party JWKS.
	authz := enroll.Authorization
	require.True(t, strings.HasPrefix(authz, "Bearer "), "enroll must carry a bearer proof")
	proof := strings.TrimSpace(strings.TrimPrefix(authz, "Bearer "))
	hdr, err := decodeHeader(proof)
	require.NoError(t, err)
	assert.Equal(t, c.Kid(), hdr.Kid, "proof must be signed with the adapter kid")
	jwks, err := gatewaytest.Ed25519JWKS(partyKey(t).Public().(ed25519.PublicKey))
	require.NoError(t, err)
	srv, err := gatewaytest.NewJWKSServer(jwks)
	require.NoError(t, err)
	t.Cleanup(srv.Close)
	_, err = srv.Verify(proof)
	require.NoError(t, err, "enrollment proof must verify against the party key")

	// Afterwards the fake serves the party JWKS with our kid, so the
	// enrolled probe (BK-PARTYID-002) reports the party enrolled.
	gw.Script(http.MethodGet, "/v1/parties/{id}/jwks", gatewaytest.ScriptStep{Body: jwks})
	enrolled, err = c.IsEnrolled(context.Background())
	require.NoError(t, err)
	assert.True(t, enrolled, "the party must be enrolled afterwards on the fake")
}

// TestRotateSubmitsNextKeysSignedWithCurrentThenSwitchesSigner covers the
// T12C contract assertion: Rotate submits the next signing key (public key
// plus derived kid) to POST /v1/parties/{id}/keys signed with the CURRENT
// key, and afterwards the adapter signs every subsequent request with the
// next key's kid.
// Parameters: t is the test harness.
// Returns: nothing; fails on any deviation.
func TestRotateSubmitsNextKeysSignedWithCurrentThenSwitchesSigner(t *testing.T) {
	gw := gatewaytest.NewGateway()
	t.Cleanup(gw.Close)

	rawURL := gw.URL() // loopback http:// — WithInsecureTransport applies under env "test"
	t.Setenv("APP_GATEWAY_URL", rawURL)
	c := newIsEnrolledAdapter(t, rawURL)
	currentKid := c.Kid()
	require.NotEmpty(t, currentKid)

	// Next keypair: seed byte i = fill+i, so seed(1) is distinct from the
	// adapter's current seed(0). enc uses the SDK registration encoding.
	nextSig := base64.StdEncoding.EncodeToString(partySeed(1))
	nextEnc := base64.StdEncoding.EncodeToString(partySeed(101))

	require.NoError(t, c.Rotate(context.Background(), nextSig, nextEnc),
		"gatewaytest fake must accept the rotation")

	// Exactly one recorded request: the rotate itself, an idempotent create
	// at POST /v1/parties/{id}/keys.
	recs := gw.Recorded()
	require.Len(t, recs, 1, "Rotate must issue exactly one request")
	rot := recs[0]
	require.Equal(t, http.MethodPost, rot.Method)
	require.Equal(t, "/v1/parties/party-test-001/keys", rot.Path)
	require.NotEmpty(t, rot.IdempotencyKey, "rotate is an idempotent create")

	// The body carries the next key material: its public half and the kid
	// the SDK derives for it.
	var body struct {
		PublicKey string `json:"public_key"`
		Kid       string `json:"kid"`
		Alg       string `json:"alg"`
	}
	require.NoError(t, json.Unmarshal(rot.Body, &body))
	nextKey := ed25519.NewKeyFromSeed(partySeed(1))
	assert.Equal(t, base64.StdEncoding.EncodeToString(
		nextKey.Public().(ed25519.PublicKey)), body.PublicKey,
		"must submit the next signing public key")
	nextKid, err := sdkKid(nextKey, "party-test-001")
	require.NoError(t, err)
	assert.Equal(t, nextKid, body.Kid, "must submit the next kid")
	assert.Equal(t, "Ed25519", body.Alg)

	// The rotation request itself is signed with the CURRENT key: its kid
	// header is the pre-rotation kid and the token verifies offline against
	// the current party JWKS.
	require.True(t, strings.HasPrefix(rot.Authorization, "Bearer "),
		"rotate must carry a bearer ks-party+jwt")
	rotTok := strings.TrimSpace(strings.TrimPrefix(rot.Authorization, "Bearer "))
	hdr, err := decodeHeader(rotTok)
	require.NoError(t, err)
	assert.Equal(t, currentKid, hdr.Kid,
		"rotate must be signed with the current key's kid")
	curJWKS, err := gatewaytest.Ed25519JWKS(
		partyKey(t).Public().(ed25519.PublicKey))
	require.NoError(t, err)
	curSrv, err := gatewaytest.NewJWKSServer(curJWKS)
	require.NoError(t, err)
	t.Cleanup(curSrv.Close)
	_, err = curSrv.Verify(rotTok)
	require.NoError(t, err, "rotate must verify against the CURRENT party key")

	// After a successful rotation the adapter signer switched: Kid() is the
	// next kid and every subsequent request carries it, verifying against
	// the NEXT key.
	assert.Equal(t, nextKid, c.Kid(),
		"the adapter must switch to the next kid after Rotate")

	resp, err := c.Do(context.Background(), Request{
		Method: http.MethodGet,
		Path:   "/v1/parties/party-test-001/health",
	})
	require.NoError(t, err, "subsequent request through the switched signer must succeed")
	require.NotNil(t, resp)
	recs = gw.Recorded()
	require.Len(t, recs, 2, "the subsequent request must reach the fake")
	nextTok := strings.TrimSpace(
		strings.TrimPrefix(recs[1].Authorization, "Bearer "))
	hdr2, err := decodeHeader(nextTok)
	require.NoError(t, err)
	assert.Equal(t, nextKid, hdr2.Kid,
		"subsequent requests must be signed with the next key's kid")
	assert.NotEqual(t, currentKid, hdr2.Kid,
		"subsequent requests must no longer carry the current kid")
	nextJWKS, err := gatewaytest.Ed25519JWKS(
		nextKey.Public().(ed25519.PublicKey))
	require.NoError(t, err)
	nextSrv, err := gatewaytest.NewJWKSServer(nextJWKS)
	require.NoError(t, err)
	t.Cleanup(nextSrv.Close)
	_, err = nextSrv.Verify(nextTok)
	require.NoError(t, err,
		"the subsequent request must verify against the NEXT party key")
}

// TestRotateFailsClosedBeforeIO proves Rotate rejects a disabled adapter and
// missing or unparseable next key material with an error before any network
// I/O (countingTransport records zero attempts).
// Parameters: t is the test harness.
// Returns: nothing; fails on any deviation.
func TestRotateFailsClosedBeforeIO(t *testing.T) {
	validNextSig := base64.StdEncoding.EncodeToString(partySeed(1))
	validNextEnc := base64.StdEncoding.EncodeToString(partySeed(101))

	t.Run("disabled adapter", func(t *testing.T) {
		var hits int
		orig := http.DefaultTransport
		http.DefaultTransport = countingTransport{hits: &hits}
		defer func() { http.DefaultTransport = orig }()

		c, err := New(config.AppGateway{}, "test", nil)
		require.NoError(t, err)
		require.False(t, c.Enabled())
		err = c.Rotate(context.Background(), validNextSig, validNextEnc)
		require.ErrorIs(t, err, contracts.ErrGatewayNotConfigured)
		assert.Equal(t, 0, hits, "disabled adapter must not touch the network")
	})

	t.Run("unparseable next signing key", func(t *testing.T) {
		var hits int
		orig := http.DefaultTransport
		http.DefaultTransport = countingTransport{hits: &hits}
		defer func() { http.DefaultTransport = orig }()

		c, err := New(config.AppGateway{
			URL: "https://gateway.example.com", PartyID: "party-test-001",
			PrivateKey: base64.StdEncoding.EncodeToString(partySeed(0)),
		}, "test", nil)
		require.NoError(t, err)
		err = c.Rotate(context.Background(), "!!!not-a-key", validNextEnc)
		require.Error(t, err, "unparseable next key must fail closed")
		assert.Equal(t, 0, hits, "bad key material must be rejected before any I/O")
	})

	t.Run("unparseable next enc key", func(t *testing.T) {
		var hits int
		orig := http.DefaultTransport
		http.DefaultTransport = countingTransport{hits: &hits}
		defer func() { http.DefaultTransport = orig }()

		c, err := New(config.AppGateway{
			URL: "https://gateway.example.com", PartyID: "party-test-001",
			PrivateKey: base64.StdEncoding.EncodeToString(partySeed(0)),
		}, "test", nil)
		require.NoError(t, err)
		err = c.Rotate(context.Background(), validNextSig, "!!!not-an-enc-key")
		require.Error(t, err, "unparseable next enc key must fail closed")
		assert.Equal(t, 0, hits, "bad key material must be rejected before any I/O")
	})
}
