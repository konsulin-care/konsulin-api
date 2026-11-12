package gateway

import (
	"context"
	"crypto/ecdh"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"konsulin-service/internal/app/config"

	"github.com/konsulin-care/gateway-sdk-go/envelope"
	"github.com/konsulin-care/gateway-sdk-go/gatewaytest"
	"github.com/konsulin-care/gateway-sdk-go/wire"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// webhook parties and listing under test.
const (
	webhookTestParty     = "party-test-001"
	webhookProviderParty = "01HZYPROVIDER00000001"
	webhookService       = "magic-link"
	webhookUserRef       = "user-ref-abc"
	webhookEncKid        = "recipient-enc-kid"
)

// newWebhookTestAdapter builds an enabled adapter against rawURL under env
// "test" (its loopback http:// URL requires WithInsecureTransport) with a
// deterministic Ed25519 seed.
// Parameters: t is the test harness, rawURL the fake gateway base URL.
// Returns: the adapter; fails the test on error.
func newWebhookTestAdapter(t *testing.T, rawURL string) *Client {
	t.Helper()
	seed := make([]byte, ed25519.SeedSize)
	for i := range seed {
		seed[i] = byte(i)
	}
	cfg := config.AppGateway{
		URL:        rawURL,
		PartyID:    webhookTestParty,
		PrivateKey: base64.StdEncoding.EncodeToString(seed),
	}
	c, err := New(cfg, "test", nil)
	require.NoError(t, err)
	require.True(t, c.Enabled())
	return c
}

// newRecipientJWKS generates the recipient's X25519 keypair and the party
// JWKS serving exactly its use:enc key.
// Parameters: t is the test harness.
// Returns: the recipient's private key and the raw JWKS document; fails the
// test on error.
func newRecipientJWKS(t *testing.T) (*ecdh.PrivateKey, []byte) {
	t.Helper()
	priv, err := ecdh.X25519().GenerateKey(rand.Reader)
	require.NoError(t, err)
	doc, err := json.Marshal(map[string]any{"keys": []map[string]string{{
		"kty": "OKP", "crv": "X25519", "use": "enc", "kid": webhookEncKid,
		"x": base64.RawURLEncoding.EncodeToString(priv.PublicKey().Bytes()),
	}}})
	require.NoError(t, err)
	return priv, doc
}

// scriptRecipientJWKS serves doc on GET /v1/parties/{id}/jwks.
// Parameters: t is the test harness, gw the fake gateway, doc the JWKS.
// Returns: nothing.
func scriptRecipientJWKS(t *testing.T, gw *gatewaytest.Gateway, doc []byte) {
	t.Helper()
	gw.Script(http.MethodGet, "/v1/parties/{id}/jwks", gatewaytest.ScriptStep{
		Status: http.StatusOK, Body: doc,
	})
}

// mustJSON marshals v for a scripted response body; fails the test on error.
// Parameters: t is the test harness, v the value to marshal.
// Returns: the JSON bytes.
func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	raw, err := json.Marshal(v)
	require.NoError(t, err)
	return raw
}

// webhookRequest finds the one recorded POST to path among recorded.
// Parameters: recorded is the fake gateway's request log, path the POST
// target.
// Returns: the recorded request and true, or false when never sent.
func webhookRequest(recorded []gatewaytest.RecordedRequest, path string) (gatewaytest.RecordedRequest, bool) {
	for _, rec := range recorded {
		if rec.Method == http.MethodPost && rec.Path == path {
			return rec, true
		}
	}
	return gatewaytest.RecordedRequest{}, false
}

// webhookBody decodes one recorded webhook POST body.
// Parameters: t is the test harness, raw the request body.
// Returns: to, service, user_ref and the payload member (must be a JSON
// string).
func decodeWebhookBody(t *testing.T, raw []byte) (to, service, userRef, payload string) {
	t.Helper()
	var body struct {
		To      string `json:"to"`
		Service string `json:"service"`
		UserRef string `json:"user_ref"`
		Payload string `json:"payload"`
	}
	require.NoError(t, json.Unmarshal(raw, &body),
		"the body must decode, so payload must be a JSON string")
	return body.To, body.Service, body.UserRef, body.Payload
}

// TestEnqueueSendsSealedPayloadRecipientOpensBound covers T29 assertion 1:
// Enqueue posts to, service, user_ref and a payload string that the
// recipient opens with OpenBound into the original data, bound from, to
// and service.
// Parameters: t is the test harness.
// Returns: nothing; fails on any deviation.
func TestEnqueueSendsSealedPayloadRecipientOpensBound(t *testing.T) {
	gw := gatewaytest.NewGateway()
	t.Cleanup(gw.Close)
	recipient, jwks := newRecipientJWKS(t)
	scriptRecipientJWKS(t, gw, jwks)
	c := newWebhookTestAdapter(t, gw.URL())

	data := []byte(`{"link":"https://app.example/ml?token=abc"}`)
	require.NoError(t, c.Enqueue(context.Background(),
		webhookProviderParty, webhookService, webhookUserRef, data),
		"the fake gateway's enqueue must be accepted")

	rec, ok := webhookRequest(gw.Recorded(), "/v1/webhooks")
	require.True(t, ok, "Enqueue must POST /v1/webhooks")
	require.NotEmpty(t, rec.IdempotencyKey,
		"a logical create must carry an Idempotency-Key")

	to, service, userRef, payload := decodeWebhookBody(t, rec.Body)
	assert.Equal(t, webhookProviderParty, to, "must send to")
	assert.Equal(t, webhookService, service, "must send service")
	assert.Equal(t, webhookUserRef, userRef, "must send user_ref")
	require.NotEmpty(t, payload, "must send a payload string")

	opened, err := envelope.OpenBound(context.Background(), payload, recipient,
		envelope.Expectations{
			From: webhookTestParty, To: webhookProviderParty, Service: webhookService,
		}, nil, time.Now)
	require.NoError(t, err, "the recipient must open the payload with OpenBound")
	assert.Equal(t, data, opened, "opened data must equal the original data")
}

// TestSyncReturnsOpenedJoseReply covers T29 assertion 2: Sync returns the
// opened data of a sealed application/jose reply.
// Parameters: t is the test harness.
// Returns: nothing; fails on any deviation.
func TestSyncReturnsOpenedJoseReply(t *testing.T) {
	gw := gatewaytest.NewGateway()
	t.Cleanup(gw.Close)
	_, jwks := newRecipientJWKS(t)
	scriptRecipientJWKS(t, gw, jwks)

	// The adapter's own enc key opens the provider's sealed reply.
	own, err := ecdh.X25519().GenerateKey(rand.Reader)
	require.NoError(t, err)
	t.Setenv("APP_PARTY_ENC_PRIVATE_KEY",
		base64.StdEncoding.EncodeToString(own.Bytes()))

	reply := []byte(`{"status":"ok","token":"xyz"}`)
	sealed, err := envelope.SealBound(reply, "provider-enc-kid", own.PublicKey(),
		envelope.Binding{
			From: webhookProviderParty, To: webhookTestParty, Service: webhookService,
		})
	require.NoError(t, err)
	gw.Script(http.MethodPost, "/v1/webhooks/sync", gatewaytest.ScriptStep{
		Status: http.StatusOK,
		Body: mustJSON(t, wire.SyncWebhookResponse{
			StatusCode: 200, Body: sealed, ContentType: wire.ContentTypeJOSE,
			WebhookID: "01JAV3Q6R7K9X2M4N8P0Q1S5T7",
		}),
	})

	c := newWebhookTestAdapter(t, gw.URL())
	data := []byte(`{"query":"cohort-a"}`)
	got, err := c.Sync(context.Background(),
		webhookProviderParty, webhookService, webhookUserRef, data)
	require.NoError(t, err, "the sealed reply must open")
	assert.Equal(t, reply, got, "must return the opened reply data")

	rec, ok := webhookRequest(gw.Recorded(), "/v1/webhooks/sync")
	require.True(t, ok, "Sync must POST /v1/webhooks/sync")
	to, service, userRef, payload := decodeWebhookBody(t, rec.Body)
	assert.Equal(t, webhookProviderParty, to, "must send to")
	assert.Equal(t, webhookService, service, "must send service")
	assert.Equal(t, webhookUserRef, userRef, "must send user_ref")
	require.NotEmpty(t, payload, "must send a payload string")
}

// TestRecipientWithoutEncKeyFailsBeforeRequest covers T29 assertion 3: a
// recipient whose JWKS carries no use:enc key fails before any webhook
// request is sent — both for Enqueue and Sync.
// Parameters: t is the test harness.
// Returns: nothing; fails on any deviation.
func TestRecipientWithoutEncKeyFailsBeforeRequest(t *testing.T) {
	// The canned party JWKS serves an EdDSA sig key only: no use:enc key.
	assertNoWebhookRequest := func(t *testing.T, gw *gatewaytest.Gateway) {
		t.Helper()
		for _, rec := range gw.Recorded() {
			assert.NotEqual(t, "/v1/webhooks", rec.Path,
				"no enqueue request may be sent")
			assert.NotEqual(t, "/v1/webhooks/sync", rec.Path,
				"no sync request may be sent")
		}
	}

	t.Run("enqueue", func(t *testing.T) {
		gw := gatewaytest.NewGateway()
		t.Cleanup(gw.Close)
		c := newWebhookTestAdapter(t, gw.URL())

		err := c.Enqueue(context.Background(),
			webhookProviderParty, webhookService, webhookUserRef, []byte(`{}`))
		require.Error(t, err, "a recipient without a use:enc key must fail")
		assertNoWebhookRequest(t, gw)
	})

	t.Run("sync", func(t *testing.T) {
		gw := gatewaytest.NewGateway()
		t.Cleanup(gw.Close)
		own, err := ecdh.X25519().GenerateKey(rand.Reader)
		require.NoError(t, err)
		t.Setenv("APP_PARTY_ENC_PRIVATE_KEY",
			base64.StdEncoding.EncodeToString(own.Bytes()))
		c := newWebhookTestAdapter(t, gw.URL())

		_, err = c.Sync(context.Background(),
			webhookProviderParty, webhookService, webhookUserRef, []byte(`{}`))
		require.Error(t, err, "a recipient without a use:enc key must fail")
		assertNoWebhookRequest(t, gw)
	})
}
