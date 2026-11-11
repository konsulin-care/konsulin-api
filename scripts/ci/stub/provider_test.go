package main

import (
	"bytes"
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/konsulin-care/gateway-sdk-go/envelope"
	"github.com/konsulin-care/gateway-sdk-go/gatewaytest"
	"github.com/konsulin-care/gateway-sdk-go/wire"
	"github.com/stretchr/testify/require"
)

// Fixed identities for the provider-party mode tests.
const (
	providerTestPartyID  = "party-provider-stub"
	providerTestSenderID = "party-sender"
	providerTestService  = "qc-webhook-stub"
)

// providerTestEnv wires the provider-party mode: a fake gateway JWKS, the
// stub's X25519 key, the caller's reply key, and a router built from env.
type providerTestEnv struct {
	mux         *http.ServeMux
	minter      *gatewaytest.Minter
	providerPub *ecdh.PublicKey
	callerPriv  *ecdh.PrivateKey
}

// newProviderTestEnv starts a fake gateway JWKS server, generates both
// X25519 keypairs, publishes the provider configuration through the
// environment, clears the recorded delivery, and builds the router.
// Parameters: t is the test harness; env vars are restored on cleanup.
// Returns: the configured test environment.
func newProviderTestEnv(t *testing.T) *providerTestEnv {
	t.Helper()
	providerLast.reset()

	minter, err := gatewaytest.NewMinter()
	require.NoError(t, err)
	jwks, err := minter.JWKS()
	require.NoError(t, err)
	srv, err := gatewaytest.NewJWKSServer(jwks)
	require.NoError(t, err)
	t.Cleanup(srv.Close)

	providerPriv, err := ecdh.X25519().GenerateKey(rand.Reader)
	require.NoError(t, err)
	callerPriv, err := ecdh.X25519().GenerateKey(rand.Reader)
	require.NoError(t, err)

	t.Setenv(providerEnvPartyID, providerTestPartyID)
	t.Setenv(providerEnvJWKSURL, srv.URL())
	t.Setenv(providerEnvEncKey, base64.StdEncoding.EncodeToString(providerPriv.Bytes()))
	t.Setenv(providerEnvReplyKey, base64.StdEncoding.EncodeToString(callerPriv.PublicKey().Bytes()))

	return &providerTestEnv{
		mux:         newRouter(),
		minter:      minter,
		providerPub: providerPriv.PublicKey(),
		callerPriv:  callerPriv,
	}
}

// sealRequest seals one bound request envelope to the stub's X25519 key.
// Parameters: t is the test harness; payload is the application payload.
// Returns: the compact JWE.
func (e *providerTestEnv) sealRequest(t *testing.T, payload string) string {
	t.Helper()
	kid, err := envelope.EncJWK(e.providerPub)
	require.NoError(t, err)
	compact, err := envelope.SealBound([]byte(payload), kid.Kid, e.providerPub, envelope.Binding{
		From:    providerTestSenderID,
		To:      providerTestPartyID,
		Service: providerTestService,
	})
	require.NoError(t, err)
	return compact
}

// mintDelivery mints a gateway-signed ks-delivery+jwt whose body_sha256
// covers the exact delivery body bytes.
// Parameters: t is the test harness; body is the request body; webhookID is
// the invocation id (webhook_id claim).
// Returns: the compact token.
func (e *providerTestEnv) mintDelivery(t *testing.T, body []byte, webhookID string) string {
	t.Helper()
	sum := sha256.Sum256(body)
	now := time.Now()
	claims, err := json.Marshal(map[string]any{
		"iss":         "gateway",
		"aud":         providerTestPartyID,
		"sub":         providerTestSenderID,
		"purpose":     wire.PurposeWebhookDelivery,
		"webhook_id":  webhookID,
		"service":     providerTestService,
		"body_sha256": base64.RawURLEncoding.EncodeToString(sum[:]),
		"iat":         now.Unix(),
		"exp":         now.Add(5 * time.Minute).Unix(),
	})
	require.NoError(t, err)
	token, err := e.minter.Mint(wire.TypDelivery, claims)
	require.NoError(t, err)
	return token
}

// postDelivery POSTs one delivery to /provider/invoke; an empty token sends
// no Authorization header, an empty webhookID no Webhook-Id header.
// Parameters: t is the test harness; token is the bearer ("" to omit);
// webhookID is the dedupe header ("" to omit); body is the request body.
// Returns: the recorded response.
func (e *providerTestEnv) postDelivery(t *testing.T, token, webhookID string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/provider/invoke", bytes.NewReader(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if webhookID != "" {
		req.Header.Set("Webhook-Id", webhookID)
	}
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.mux.ServeHTTP(rec, req)
	return rec
}

// getLast requests GET /provider/last.
// Parameters: t is the test harness.
// Returns: the recorded response.
func (e *providerTestEnv) getLast(t *testing.T) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/provider/last", nil)
	rec := httptest.NewRecorder()
	e.mux.ServeHTTP(rec, req)
	return rec
}

// TestProviderDeliveryValidTokenRecordsPayload covers the contract: a valid
// gateway-signed delivery is answered 200 and its opened payload is
// returned by GET /provider/last.
// Parameters: t is the test harness.
// Returns nothing; fails on any deviation.
func TestProviderDeliveryValidTokenRecordsPayload(t *testing.T) {
	env := newProviderTestEnv(t)
	const payload = `{"template":"qc"}`
	compact := env.sealRequest(t, payload)
	body, err := json.Marshal(wire.DeliveryBody{Envelope: compact, ResultTicket: "tk-provider-1"})
	require.NoError(t, err)
	token := env.mintDelivery(t, body, "wh-provider-1")

	rec := env.postDelivery(t, token, "wh-provider-1", body)
	require.Equal(t, http.StatusOK, rec.Code,
		"a valid gateway-signed delivery must be answered 200")

	last := env.getLast(t)
	require.Equal(t, http.StatusOK, last.Code, "the opened payload must be served at GET /provider/last")
	var got providerRecord
	require.NoError(t, json.Unmarshal(last.Body.Bytes(), &got))
	require.Equal(t, payload, got.Payload, "the opened payload must be the sealed application payload")
	require.Equal(t, providerTestService, got.Service)
	require.Equal(t, "wh-provider-1", got.WebhookID)
}

// TestProviderDeliveryMissingOrInvalidTokenUnauthorized covers the contract:
// a delivery with a missing or invalid token is answered 401 and nothing is
// recorded.
// Parameters: t is the test harness.
// Returns nothing; fails on any deviation.
func TestProviderDeliveryMissingOrInvalidTokenUnauthorized(t *testing.T) {
	env := newProviderTestEnv(t)
	compact := env.sealRequest(t, `{"template":"qc"}`)
	body, err := json.Marshal(wire.DeliveryBody{Envelope: compact, ResultTicket: "tk-provider-2"})
	require.NoError(t, err)

	missing := env.postDelivery(t, "", "wh-missing-1", body)
	require.Equal(t, http.StatusUnauthorized, missing.Code,
		"a delivery without a token must be answered 401")

	invalid := env.postDelivery(t, "not-a-jwt", "wh-invalid-1", body)
	require.Equal(t, http.StatusUnauthorized, invalid.Code,
		"a delivery with an invalid token must be answered 401")

	last := env.getLast(t)
	require.Equal(t, http.StatusNotFound, last.Code,
		"nothing may be recorded after a denied delivery")
}

// TestProviderSyncDeliveryRepliesSealed covers the sync branch: a delivery
// without a result ticket answers with a bound envelope sealed to the
// caller's use:enc key (Content-Type application/jose) and still records
// the opened payload.
// Parameters: t is the test harness.
// Returns nothing; fails on any deviation.
func TestProviderSyncDeliveryRepliesSealed(t *testing.T) {
	env := newProviderTestEnv(t)
	const payload = `{"template":"qc-sync"}`
	compact := env.sealRequest(t, payload)
	body, err := json.Marshal(wire.DeliveryBody{Envelope: compact})
	require.NoError(t, err)
	token := env.mintDelivery(t, body, "wh-sync-1")

	rec := env.postDelivery(t, token, "wh-sync-1", body)
	require.Equal(t, http.StatusOK, rec.Code, "a sync delivery must be answered 200")
	require.Equal(t, "application/jose", rec.Header().Get("Content-Type"),
		"a sync reply must be a compact JWE")

	data, err := envelope.OpenBound(context.Background(), rec.Body.String(), env.callerPriv,
		envelope.Expectations{
			From:    providerTestPartyID,
			To:      providerTestSenderID,
			Service: providerTestService,
		},
		nil, time.Now)
	require.NoError(t, err, "the reply must open with the caller's key and swapped bindings")
	var reply struct {
		Status    string `json:"status"`
		Service   string `json:"service"`
		WebhookID string `json:"webhook_id"`
	}
	require.NoError(t, json.Unmarshal(data, &reply))
	require.Equal(t, "ok", reply.Status)
	require.Equal(t, providerTestService, reply.Service)
	require.Equal(t, "wh-sync-1", reply.WebhookID)

	last := env.getLast(t)
	require.Equal(t, http.StatusOK, last.Code, "the opened payload must be recorded on sync too")
	var got providerRecord
	require.NoError(t, json.Unmarshal(last.Body.Bytes(), &got))
	require.Equal(t, payload, got.Payload)
}
