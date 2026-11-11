package gateway

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/konsulin-care/gateway-sdk-go/gatewaytest"
	"github.com/konsulin-care/gateway-sdk-go/verify"
	"github.com/konsulin-care/gateway-sdk-go/wire"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// verifyEnv wires one fake gateway JWKS endpoint, one miniredis and the
// environment VerifyMiddleware reads, and mints gateway-signed tokens with
// the fake gateway key.
type verifyEnv struct {
	minter *gatewaytest.Minter
	self   string
	mr     *miniredis.Miniredis
}

// newVerifyEnv stands up the fake gateway JWKS server and miniredis, points
// APP_GATEWAY_URL / APP_PARTY_ID / REDIS_* at them, and builds the
// VerifyMiddleware for kinds (default: ks-event+jwt and ks-delivery+jwt).
// The servers close and the env vars restore automatically on cleanup.
//
// Parameters: t is the test harness; kinds are the token kinds under test.
// Returns: the token-minting env and the built middleware. Fails on error.
func newVerifyEnv(t *testing.T, kinds ...string) (*verifyEnv, func(http.Handler) http.Handler) {
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

	self := "pty-test-self"
	t.Setenv("APP_GATEWAY_URL", srv.URL())
	t.Setenv("APP_PARTY_ID", self)

	if len(kinds) == 0 {
		kinds = []string{wire.TypEvent, wire.TypDelivery}
	}
	mw, err := VerifyMiddleware(kinds...)
	require.NoError(t, err)
	return &verifyEnv{minter: minter, self: self, mr: mr}, mw
}

// mint signs claims as a compact token of typ with the fake gateway key.
//
// Parameters: t is the test harness; typ is the token kind header value;
// claims is the JSON claims object.
// Returns: the compact JWS. Fails on any minting error.
func (e *verifyEnv) mint(t *testing.T, typ string, claims map[string]any) string {
	t.Helper()
	payload, err := json.Marshal(claims)
	require.NoError(t, err)
	tok, err := e.minter.Mint(typ, payload)
	require.NoError(t, err)
	return tok
}

// gatewayClaims returns the claim skeleton every gateway-signed kind
// carries: iss gateway, aud this party's id, a sub, and an iat/exp pair
// around the wall clock.
func (e *verifyEnv) gatewayClaims() map[string]any {
	now := time.Now()
	return map[string]any{
		"iss": "gateway",
		"aud": e.self,
		"sub": "pty-test-sender",
		"iat": now.Add(-time.Minute).Unix(),
		"exp": now.Add(5 * time.Minute).Unix(),
	}
}

// deliveryClaims returns a well-formed ks-delivery+jwt claims object whose
// body_sha256 covers body and which carries the conditional service and
// listing_id claims.
//
// Parameters: webhookID is the webhook invocation id; body is the exact
// request body bytes the digest covers.
// Returns: the claims object to mint.
func (e *verifyEnv) deliveryClaims(webhookID string, body []byte) map[string]any {
	c := e.gatewayClaims()
	c["purpose"] = wire.PurposeWebhookDelivery
	c["webhook_id"] = webhookID
	c["body_sha256"] = shaClaim(body)
	c["service"] = "konsulin-calendar"
	c["listing_id"] = "lst-test-001"
	return c
}

// shaClaim renders the body_sha256 claim value over body: the base64url
// (unpadded) SHA-256 digest the SDK verifies against the exact bytes.
//
// Parameters: body is the exact request body bytes.
// Returns: the claim value.
func shaClaim(body []byte) string {
	sum := sha256.Sum256(body)
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// newCallbackRequest builds a gateway callback request carrying the Bearer
// token (skipped when empty), the Webhook-Id header (skipped when empty),
// and the exact body bytes.
//
// Parameters: tok is the bearer token, webhookID the transport header value,
// body the request body.
// Returns: the request ready to serve.
func newCallbackRequest(tok, webhookID string, body []byte) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/webhooks/gateway", bytes.NewReader(body))
	if tok != "" {
		r.Header.Set("Authorization", "Bearer "+tok)
	}
	if webhookID != "" {
		r.Header.Set("Webhook-Id", webhookID)
	}
	return r
}

// TestVerifyMiddlewareDeniesInvalidToken covers assertion 1: a request
// without a valid gateway-signed token is rejected 401 and the next handler
// is not called. Missing bearer credentials, a malformed token, and a token
// signed by a foreign key (unknown kid against the gateway JWKS) all deny
// alike.
//
// Parameters: t is the test harness.
// Returns: nothing; fails on any deviation.
func TestVerifyMiddlewareDeniesInvalidToken(t *testing.T) {
	env, mw := newVerifyEnv(t)
	body := []byte(`{"event":"payment.paid"}`)

	foreign, err := gatewaytest.NewMinter()
	require.NoError(t, err)
	payload, err := json.Marshal(env.deliveryClaims("wh-foreign", body))
	require.NoError(t, err)
	foreignTok, err := foreign.Mint(wire.TypDelivery, payload)
	require.NoError(t, err)

	cases := map[string]string{
		"missing authorization": "",
		"malformed token":       "not-a-jws",
		"foreign signing key":   foreignTok,
	}
	for name, tok := range cases {
		t.Run(name, func(t *testing.T) {
			var calls int
			h := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls++
				w.WriteHeader(http.StatusNoContent)
			}))
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, newCallbackRequest(tok, "wh-1", body))
			assert.Equal(t, http.StatusUnauthorized, rec.Code,
				"a request without a valid gateway-signed token must be rejected 401")
			assert.Zero(t, calls, "the next handler must not be called")
		})
	}
}

// TestVerifyMiddlewareDuplicateWebhookIDAcknowledged covers assertion 2: a
// repeated Webhook-Id is acknowledged 200 without calling the next handler,
// deduplicated through the Redis replay cache (T06). The first delivery
// reserves gw:replay:webhook_id:<id> in Redis; the gateway's retry — a
// re-minted token with the same webhook_id — answers 200 with an empty body
// and never reaches the handler.
//
// Parameters: t is the test harness.
// Returns: nothing; fails on any deviation.
func TestVerifyMiddlewareDuplicateWebhookIDAcknowledged(t *testing.T) {
	env, mw := newVerifyEnv(t)
	body := []byte(`{"id":"evt-1"}`)
	var calls int
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(http.StatusNoContent)
	}))

	tok := env.mint(t, wire.TypDelivery, env.deliveryClaims("wh-dup-1", body))
	first := httptest.NewRecorder()
	h.ServeHTTP(first, newCallbackRequest(tok, "wh-dup-1", body))
	assert.Equal(t, http.StatusNoContent, first.Code,
		"the first delivery must reach the next handler")
	assert.Equal(t, 1, calls)
	require.True(t, env.mr.Exists("gw:replay:webhook_id:wh-dup-1"),
		"the reservation must live in the T06 Redis replay cache")

	retry := env.mint(t, wire.TypDelivery, env.deliveryClaims("wh-dup-1", body))
	second := httptest.NewRecorder()
	h.ServeHTTP(second, newCallbackRequest(retry, "wh-dup-1", body))
	assert.Equal(t, http.StatusOK, second.Code,
		"a repeated Webhook-Id must be acknowledged 200")
	assert.Empty(t, second.Body.String(), "the acknowledgement must carry an empty body")
	assert.Equal(t, 1, calls, "a repeated Webhook-Id must not call the next handler")
}

// TestVerifyMiddlewareClaimsFromContext covers assertion 3: verified claims
// are readable from the request context in the next handler — purpose, sub,
// service, listing_id and the webhook id of the authorizing token — for
// both ks-delivery+jwt and ks-event+jwt dispatch.
//
// Parameters: t is the test harness.
// Returns: nothing; fails on any deviation.
func TestVerifyMiddlewareClaimsFromContext(t *testing.T) {
	env, mw := newVerifyEnv(t)

	t.Run("ks-delivery+jwt", func(t *testing.T) {
		body := []byte(`{"hello":"world"}`)
		var got verify.Claims
		h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			got = ClaimsFromContext(r.Context())
			w.WriteHeader(http.StatusNoContent)
		}))
		tok := env.mint(t, wire.TypDelivery, env.deliveryClaims("wh-claims-1", body))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, newCallbackRequest(tok, "wh-claims-1", body))
		require.Equal(t, http.StatusNoContent, rec.Code,
			"a valid delivery must reach the next handler")
		assert.Equal(t, wire.TypDelivery, got.Typ)
		assert.Equal(t, wire.PurposeWebhookDelivery, got.Purpose)
		assert.Equal(t, "pty-test-sender", got.Sub)
		assert.Equal(t, "konsulin-calendar", got.Service)
		assert.Equal(t, "lst-test-001", got.ListingID)
		assert.Equal(t, "wh-claims-1", got.WebhookID)
	})

	t.Run("ks-event+jwt", func(t *testing.T) {
		body := []byte(`{"event":"settlement.paid"}`)
		var got verify.Claims
		h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			got = ClaimsFromContext(r.Context())
			w.WriteHeader(http.StatusNoContent)
		}))
		claims := env.gatewayClaims()
		claims["purpose"] = wire.PurposeSettlementNotification
		claims["webhook_id"] = "wh-claims-2"
		claims["body_sha256"] = shaClaim(body)
		tok := env.mint(t, wire.TypEvent, claims)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, newCallbackRequest(tok, "wh-claims-2", body))
		require.Equal(t, http.StatusNoContent, rec.Code,
			"a valid event must reach the next handler")
		assert.Equal(t, wire.TypEvent, got.Typ)
		assert.Equal(t, wire.PurposeSettlementNotification, got.Purpose)
		assert.Equal(t, "pty-test-sender", got.Sub)
		assert.Equal(t, "wh-claims-2", got.WebhookID)
	})
}
