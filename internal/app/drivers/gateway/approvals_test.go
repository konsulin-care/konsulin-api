package gateway

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"net"
	"net/http"
	"testing"
	"time"

	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/contracts"

	"github.com/alicebob/miniredis/v2"
	"github.com/konsulin-care/gateway-sdk-go/gatewaytest"
	"github.com/konsulin-care/gateway-sdk-go/verify"
	"github.com/konsulin-care/gateway-sdk-go/wire"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// approvalCannedTicket is the canned access_token the fake gateway serves
// for POST /v1/approvals.
const approvalCannedTicket = "eyJhbGciOiJFZERTQSIsImtpZCI6InFxLXBhcnR5LWtpZC0xIn0.cGF5bG9hZA.c2ln"

// researcherHashFixture is the fixed base64 32-byte researcher hash the
// mint tests post — test-only material, never a real researcher identity.
const researcherHashFixture = "MDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDA="

// newApprovalTestAdapter builds an enabled adapter against rawURL under
// env "test" with a deterministic Ed25519 seed (byte i = i).
// Parameters: t is the test harness, rawURL the fake gateway base URL.
// Returns: the adapter; fails the test on error.
func newApprovalTestAdapter(t *testing.T, rawURL string) *Client {
	t.Helper()
	seed := make([]byte, ed25519.SeedSize)
	for i := range seed {
		seed[i] = byte(i)
	}
	cfg := config.AppGateway{
		URL:        rawURL,
		PartyID:    "party-test-001",
		PrivateKey: base64.StdEncoding.EncodeToString(seed),
	}
	c, err := New(cfg, "test", nil)
	require.NoError(t, err)
	require.True(t, c.Enabled())
	return c
}

// approvalVerifyEnv stands up the fake gateway JWKS server and miniredis,
// points APP_GATEWAY_URL / APP_PARTY_ID / REDIS_* at them, and builds an
// enabled adapter so VerifyTicket resolves its verifier from the same
// environment. Servers and env vars clean up automatically.
type approvalVerifyEnv struct {
	c      *Client
	minter *gatewaytest.Minter
	srv    *gatewaytest.JWKSServer
	self   string
}

// newApprovalVerifyEnv wires the JWKS server, miniredis and environment
// VerifyTicket reads.
// Parameters: t is the test harness.
// Returns: the verification env; fails the test on error.
func newApprovalVerifyEnv(t *testing.T) *approvalVerifyEnv {
	t.Helper()
	minter, err := gatewaytest.NewMinter()
	require.NoError(t, err)
	jwks, err := minter.JWKS()
	require.NoError(t, err)
	srv, err := gatewaytest.NewJWKSServer(jwks)
	require.NoError(t, err)
	t.Cleanup(srv.Close)

	mr := miniredis.RunT(t)
	host, port, err := net.SplitHostPort(mr.Addr())
	require.NoError(t, err)
	t.Setenv("REDIS_HOST", host)
	t.Setenv("REDIS_PORT", port)
	t.Setenv("REDIS_PASSWORD", "")

	self := "party-test-001"
	t.Setenv("APP_GATEWAY_URL", srv.URL())
	t.Setenv("APP_PARTY_ID", self)

	return &approvalVerifyEnv{
		c:      newApprovalTestAdapter(t, srv.URL()),
		minter: minter,
		srv:    srv,
		self:   self,
	}
}

// approvalClaims returns the claim set of a well-formed ks-approval+jwt:
// iss gateway, aud this party, the required scope/purpose/researcher_hash
// and jti/iat/exp around the wall clock.
// Parameters: jti the ticket id, scope the scope claim.
// Returns: the claims object to mint.
func (e *approvalVerifyEnv) approvalClaims(jti, scope string) map[string]any {
	now := time.Now()
	return map[string]any{
		"iss":             "gateway",
		"aud":             e.self,
		"sub":             "pty-test-sender",
		"scope":           scope,
		"purpose":         "referral-handoff",
		"researcher_hash": "N/A",
		"jti":             jti,
		"iat":             now.Add(-time.Minute).Unix(),
		"exp":             now.Add(5 * time.Minute).Unix(),
	}
}

// mintApproval signs claims as a ks-approval+jwt with the fake gateway key.
// Parameters: t is the test harness, claims the JSON claims object.
// Returns: the compact JWS; fails the test on minting error.
func (e *approvalVerifyEnv) mintApproval(t *testing.T, claims map[string]any) string {
	t.Helper()
	payload, err := json.Marshal(claims)
	require.NoError(t, err)
	tok, err := e.minter.Mint(wire.TypApproval, payload)
	require.NoError(t, err)
	return tok
}

// TestMintPostsScopePurposeAndResearcherHash covers T62 assertion 1: Mint
// posts scope, purpose and researcher_hash to POST /v1/approvals as an
// idempotent create and returns the minted ticket; a disabled adapter
// fails closed before any I/O.
// Parameters: t is the test harness.
// Returns: nothing; fails on any deviation.
func TestMintPostsScopePurposeAndResearcherHash(t *testing.T) {
	gw := gatewaytest.NewGateway()
	t.Cleanup(gw.Close)
	c := newApprovalTestAdapter(t, gw.URL())

	ticket, err := c.Mint(context.Background(),
		"pty-recipient-001", "referral", "cardiology-handoff", researcherHashFixture)
	require.NoError(t, err, "the fake gateway's mint must be accepted")
	assert.Equal(t, approvalCannedTicket, ticket,
		"must return the minted ticket")

	recs := gw.Recorded()
	require.Len(t, recs, 1, "one mint must issue exactly one request")
	require.Equal(t, http.MethodPost, recs[0].Method)
	require.Equal(t, "/v1/approvals", recs[0].Path)
	require.NotEmpty(t, recs[0].IdempotencyKey,
		"a logical create must carry an Idempotency-Key")

	var body map[string]any
	require.NoError(t, json.Unmarshal(recs[0].Body, &body), "body must be JSON")
	assert.Equal(t, "pty-recipient-001", body["to"], "must send the recipient party")
	assert.Equal(t, "referral", body["scope"], "must send the scope")
	assert.Equal(t, "cardiology-handoff", body["purpose"], "must send the purpose")
	assert.Equal(t, researcherHashFixture, body["researcher_hash"],
		"must send the researcher hash")

	t.Run("disabled adapter fails closed", func(t *testing.T) {
		off, err := New(config.AppGateway{}, "test", nil)
		require.NoError(t, err)
		_, err = off.Mint(context.Background(), "pty-recipient-001",
			"referral", "cardiology-handoff", researcherHashFixture)
		require.ErrorIs(t, err, contracts.ErrGatewayNotConfigured,
			"a disabled adapter must fail closed before any I/O")
	})
}

// TestVerifyTicketAcceptsGatewayTicketForExpectedScope covers T62
// assertion 2: VerifyTicket accepts a valid gateway-signed ks-approval+jwt
// for the expected scope — signature proven offline against the fake
// gateway JWKS, then the verifier's rules consume the ticket's jti in the
// Redis replay cache.
// Parameters: t is the test harness.
// Returns: nothing; fails on any deviation.
func TestVerifyTicketAcceptsGatewayTicketForExpectedScope(t *testing.T) {
	env := newApprovalVerifyEnv(t)
	tok := env.mintApproval(t, env.approvalClaims("jti-accept-1", "referral"))

	_, err := env.srv.Verify(tok)
	require.NoError(t, err,
		"the ticket must be a real gateway-signed JWS (offline proof)")

	err = env.c.VerifyTicket(context.Background(), tok, "referral")
	require.NoError(t, err,
		"a valid gateway-signed ticket for the expected scope must be accepted")
}

// TestVerifyTicketRejectsReplayAudienceScope covers T62 assertion 3: a
// replayed jti, a wrong audience or a wrong scope is rejected — each denial
// carries the matching SDK sentinel and never accepts.
// Parameters: t is the test harness.
// Returns: nothing; fails on any deviation.
func TestVerifyTicketRejectsReplayAudienceScope(t *testing.T) {
	env := newApprovalVerifyEnv(t)
	ctx := context.Background()

	t.Run("replayed jti is rejected", func(t *testing.T) {
		tok := env.mintApproval(t, env.approvalClaims("jti-replay-1", "referral"))
		require.NoError(t, env.c.VerifyTicket(ctx, tok, "referral"),
			"the first presentation must be accepted and consume the jti")
		err := env.c.VerifyTicket(ctx, tok, "referral")
		require.Error(t, err, "a replayed ticket must be rejected")
		assert.ErrorIs(t, err, verify.ErrReplay,
			"a replayed jti must deny with ErrReplay")
	})

	t.Run("wrong audience is rejected", func(t *testing.T) {
		claims := env.approvalClaims("jti-aud-1", "referral")
		claims["aud"] = "pty-other-recipient"
		tok := env.mintApproval(t, claims)
		err := env.c.VerifyTicket(ctx, tok, "referral")
		require.Error(t, err, "a ticket for another audience must be rejected")
		assert.ErrorIs(t, err, verify.ErrAudience,
			"a wrong audience must deny with ErrAudience")
	})

	t.Run("wrong scope is rejected", func(t *testing.T) {
		tok := env.mintApproval(t, env.approvalClaims("jti-scope-1", "referral"))
		err := env.c.VerifyTicket(ctx, tok, "export:study-x")
		require.Error(t, err, "a scope mismatch must be rejected")
		assert.ErrorIs(t, err, verify.ErrClaims,
			"a wrong scope must deny with ErrClaims")
	})

	t.Run("disabled adapter fails closed", func(t *testing.T) {
		off, err := New(config.AppGateway{}, "test", nil)
		require.NoError(t, err)
		err = off.VerifyTicket(ctx, "not-a-token", "referral")
		require.ErrorIs(t, err, contracts.ErrGatewayNotConfigured,
			"a disabled adapter must fail closed before any I/O")
	})
}
