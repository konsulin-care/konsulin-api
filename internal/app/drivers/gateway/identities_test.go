package gateway

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/konsulin-care/konsulin-api/internal/app/config"
	"github.com/konsulin-care/konsulin-api/internal/app/contracts"

	"github.com/konsulin-care/gateway-sdk-go/gatewaytest"
	"github.com/konsulin-care/gateway-sdk-go/identity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// identityPepper is the fixed 32-byte test pepper the identity hashes are
// keyed with — test-only material, never a production pepper.
var identityPepper = []byte("0123456789abcdef0123456789abcdef")

// newIdentityTestAdapter builds an enabled adapter against rawURL under env
// "test" with a deterministic Ed25519 seed (byte i = i).
// Parameters: t is the test harness, rawURL the fake gateway base URL.
// Returns: the adapter; fails the test on error.
func newIdentityTestAdapter(t *testing.T, rawURL string) *Client {
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

// identityTestHashes returns the peppered email and phone hashes for the
// fixed identity fixture used across the identity tests.
// Parameters: none.
// Returns: the hashfmt email hash and phone hash; fails on hashing error.
func identityTestHashes(t *testing.T) (string, string) {
	t.Helper()
	emailHash, err := identity.HashEmail("Person@Example.COM", identityPepper)
	require.NoError(t, err)
	phoneHash, err := identity.HashPhone("6281234567890", identityPepper)
	require.NoError(t, err)
	return emailHash, phoneHash
}

// TestRegisterSendsOnlyPepperedHashesForSubject covers T43 assertion 1:
// Register posts exactly the peppered email and phone hashes for the
// party-local subject to POST /v1/identities/register — an idempotent create
// carrying an Idempotency-Key — and a plaintext field fails closed before
// any request is sent.
// Parameters: t is the test harness.
// Returns: nothing; fails on any deviation.
func TestRegisterSendsOnlyPepperedHashesForSubject(t *testing.T) {
	gw := gatewaytest.NewGateway()
	t.Cleanup(gw.Close)
	c := newIdentityTestAdapter(t, gw.URL())
	emailHash, phoneHash := identityTestHashes(t)

	globalKey, err := c.Register(context.Background(),
		"patient", "Patient/01HZYPATIENT000001", emailHash, phoneHash)
	require.NoError(t, err, "the fake gateway's register must be accepted")
	assert.Equal(t, "01HZYGLOBALKEY000001", globalKey,
		"must return the gateway global_key")

	recs := gw.Recorded()
	require.Len(t, recs, 1, "one register must issue exactly one request")
	require.Equal(t, http.MethodPost, recs[0].Method)
	require.Equal(t, "/v1/identities/register", recs[0].Path)
	require.NotEmpty(t, recs[0].IdempotencyKey,
		"a logical create must carry an Idempotency-Key")

	var body map[string]any
	require.NoError(t, json.Unmarshal(recs[0].Body, &body), "body must be JSON")
	assert.Equal(t, "patient", body["resource_type"], "must send resource_type")
	assert.Equal(t, "Patient/01HZYPATIENT000001", body["local_resource_id"],
		"must send the subject's local resource id")
	assert.Equal(t, emailHash, body["email_hash"],
		"must send the peppered email hash")
	assert.Equal(t, phoneHash, body["phone_hash"],
		"must send the peppered phone hash")

	raw := strings.ToLower(string(recs[0].Body))
	assert.NotContains(t, raw, "person@example.com",
		"the plaintext email must never be sent")
	assert.NotContains(t, raw, "6281234567890",
		"the plaintext phone must never be sent")

	t.Run("plaintext field fails closed before any I/O", func(t *testing.T) {
		_, err := c.Register(context.Background(), "patient",
			"Patient/01HZYPATIENT000001", "person@example.com", phoneHash)
		require.Error(t, err, "a non-hashfmt field must fail closed")
		assert.Len(t, gw.Recorded(), 1,
			"no request may be sent for a non-hashfmt field")
	})
}

// TestResolveAndLinkSendHashedFieldsAndMapErrors covers T43 assertion 2:
// Resolve sends the hashed fields as a read (no Idempotency-Key) and Link
// sends the confirmed global key as an idempotent create; both map gateway
// failures through MapError keeping the HTTP status, GW-* code and
// request_id, and a disabled adapter fails closed with
// ErrGatewayNotConfigured (503 through MapError).
// Parameters: t is the test harness.
// Returns: nothing; fails on any deviation.
func TestResolveAndLinkSendHashedFieldsAndMapErrors(t *testing.T) {
	t.Run("resolve sends hashed fields", func(t *testing.T) {
		gw := gatewaytest.NewGateway()
		t.Cleanup(gw.Close)
		c := newIdentityTestAdapter(t, gw.URL())
		emailHash, phoneHash := identityTestHashes(t)

		cands, err := c.Resolve(context.Background(), emailHash, phoneHash)
		require.NoError(t, err, "the fake gateway's resolve must be accepted")
		require.Len(t, cands, 1, "must return the gateway's candidates")
		assert.Equal(t, "01HZYGLOBALKEY000001", cands[0].GlobalKey)
		assert.Equal(t, "01HZYPARTY0000000000", cands[0].PartyID)
		assert.Equal(t, "fixture-patient-001", cands[0].LocalResourceID)
		assert.Equal(t, "email", cands[0].MatchField)

		recs := gw.Recorded()
		require.Len(t, recs, 1, "one resolve must issue exactly one request")
		require.Equal(t, http.MethodPost, recs[0].Method)
		require.Equal(t, "/v1/identities/resolve", recs[0].Path)
		assert.Empty(t, recs[0].IdempotencyKey,
			"a read must not send an Idempotency-Key")

		var body map[string]any
		require.NoError(t, json.Unmarshal(recs[0].Body, &body), "body must be JSON")
		assert.Equal(t, emailHash, body["email_hash"],
			"must send the peppered email hash")
		assert.Equal(t, phoneHash, body["phone_hash"],
			"must send the peppered phone hash")
		raw := strings.ToLower(string(recs[0].Body))
		assert.NotContains(t, raw, "person@example.com",
			"the plaintext email must never be sent")
		assert.NotContains(t, raw, "6281234567890",
			"the plaintext phone must never be sent")
	})

	t.Run("link sends the confirmed global key", func(t *testing.T) {
		gw := gatewaytest.NewGateway()
		t.Cleanup(gw.Close)
		c := newIdentityTestAdapter(t, gw.URL())

		linked, err := c.Link(context.Background(),
			"01HZYGLOBALKEY000001", "Patient/01HZYPATIENT000001")
		require.NoError(t, err, "the fake gateway's link must be accepted")
		assert.True(t, linked, "must return the gateway's link result")

		recs := gw.Recorded()
		require.Len(t, recs, 1, "one link must issue exactly one request")
		require.Equal(t, http.MethodPost, recs[0].Method)
		require.Equal(t, "/v1/identities/link", recs[0].Path)
		require.NotEmpty(t, recs[0].IdempotencyKey,
			"a logical create must carry an Idempotency-Key")

		var body map[string]any
		require.NoError(t, json.Unmarshal(recs[0].Body, &body), "body must be JSON")
		assert.Equal(t, "01HZYGLOBALKEY000001", body["global_key"],
			"must send the confirmed global key")
		assert.Equal(t, "Patient/01HZYPATIENT000001", body["local_resource_id"],
			"must send the subject's local resource id")
	})

	t.Run("resolve gateway error maps through MapError", func(t *testing.T) {
		gw := gatewaytest.NewGateway()
		t.Cleanup(gw.Close)
		gw.Script(http.MethodPost, "/v1/identities/resolve", gatewaytest.ScriptStep{
			Status: http.StatusConflict,
			Body: []byte(`{"error":{"code":"GW-ID-002",` +
				`"message":"identity resolve refused","request_id":"gw-req-7"}}`),
		})
		c := newIdentityTestAdapter(t, gw.URL())
		emailHash, phoneHash := identityTestHashes(t)

		_, err := c.Resolve(context.Background(), emailHash, phoneHash)
		require.Error(t, err, "a gateway failure must surface as an error")
		ce := MapError(err)
		require.NotNil(t, ce, "resolve errors must map through MapError")
		assert.Equal(t, http.StatusConflict, ce.StatusCode,
			"the gateway HTTP status must be kept")
		assert.Contains(t, ce.DevMessage, "GW-ID-002",
			"dev_message must carry the GW-* code")
		assert.Contains(t, ce.DevMessage, "gw-req-7",
			"dev_message must carry the gateway request_id")
	})

	t.Run("link gateway error maps through MapError", func(t *testing.T) {
		gw := gatewaytest.NewGateway()
		t.Cleanup(gw.Close)
		gw.Script(http.MethodPost, "/v1/identities/link", gatewaytest.ScriptStep{
			Status: http.StatusForbidden,
			Body: []byte(`{"error":{"code":"GW-ID-004",` +
				`"message":"link refused","request_id":"gw-req-9"}}`),
		})
		c := newIdentityTestAdapter(t, gw.URL())

		_, err := c.Link(context.Background(),
			"01HZYGLOBALKEY000001", "Patient/01HZYPATIENT000001")
		require.Error(t, err, "a gateway failure must surface as an error")
		ce := MapError(err)
		require.NotNil(t, ce, "link errors must map through MapError")
		assert.Equal(t, http.StatusForbidden, ce.StatusCode,
			"the gateway HTTP status must be kept")
		assert.Contains(t, ce.DevMessage, "GW-ID-004",
			"dev_message must carry the GW-* code")
		assert.Contains(t, ce.DevMessage, "gw-req-9",
			"dev_message must carry the gateway request_id")
	})

	t.Run("disabled adapter fails closed", func(t *testing.T) {
		c, err := New(config.AppGateway{}, "test", nil)
		require.NoError(t, err)
		emailHash, phoneHash := identityTestHashes(t)

		_, err = c.Register(context.Background(), "patient",
			"Patient/01HZYPATIENT000001", emailHash, phoneHash)
		require.ErrorIs(t, err, contracts.ErrGatewayNotConfigured,
			"a disabled adapter must fail closed before any I/O")
		_, err = c.Resolve(context.Background(), emailHash, phoneHash)
		require.ErrorIs(t, err, contracts.ErrGatewayNotConfigured,
			"a disabled adapter must fail closed before any I/O")
		_, err = c.Link(context.Background(),
			"01HZYGLOBALKEY000001", "Patient/01HZYPATIENT000001")
		require.ErrorIs(t, err, contracts.ErrGatewayNotConfigured,
			"a disabled adapter must fail closed before any I/O")

		ce := MapError(err)
		require.NotNil(t, ce, "the failure must map through MapError")
		assert.Equal(t, http.StatusServiceUnavailable, ce.StatusCode,
			"a missing gateway must answer 503")
	})
}
