package main

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"konsulin-service/internal/pkg/exceptions"

	"github.com/konsulin-care/gateway-sdk-go/gatewaytest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// enlistEnv points the CLI at the given gatewaytest fake with a fixed
// current party keypair (seed byte i = i, so seed(0) is deterministic).
// Parameters: t is the test harness, gw the fake gateway.
// Returns: the current signing key. Never panics.
func enlistEnv(t *testing.T, gw *gatewaytest.Gateway) ed25519.PrivateKey {
	t.Helper()
	seed := make([]byte, ed25519.SeedSize)
	for i := range seed {
		seed[i] = byte(i)
	}
	encSeed := bytes.Repeat([]byte{0x42}, 32)
	t.Setenv("APP_ENV", "test")
	t.Setenv("APP_GATEWAY_URL", gw.URL())
	t.Setenv("APP_PARTY_ID", "party-test-001")
	t.Setenv("APP_PARTY_PRIVATE_KEY", base64.StdEncoding.EncodeToString(seed))
	t.Setenv("APP_PARTY_ENC_PRIVATE_KEY", base64.StdEncoding.EncodeToString(encSeed))
	return ed25519.NewKeyFromSeed(seed)
}

// TestEnlistSubmitsListingFieldsSignedByPartyKeyAndPrintsListingID covers
// the T47 contract assertion: enlist submits the listing fields signed by
// the party key and prints the listing_id.
// Parameters: t is the test harness.
// Returns: nothing; fails on any deviation.
func TestEnlistSubmitsListingFieldsSignedByPartyKeyAndPrintsListingID(t *testing.T) {
	gw := gatewaytest.NewGateway()
	t.Cleanup(gw.Close)
	currentKey := enlistEnv(t, gw)

	var stdout, stderr bytes.Buffer
	require.NoError(t, run([]string{
		"enlist",
		"--name", "send-magiclink",
		"--description", "Deliver a magic login link",
		"--endpoint", "https://n8n.example.com/webhook/send-magiclink",
		"--price", "53200",
		"--invocation", "webhook",
		"--price-scope", "party",
	}, &stdout, &stderr), "enlist must succeed against the gatewaytest fake")
	require.Empty(t, stderr.String(), "enlist must not write to stderr")
	assert.Contains(t, stdout.String(), "listing_id=01HZYLISTING000001",
		"enlist must print the listing_id returned by the gateway")

	// Exactly one request reached the fake: the create itself.
	recs := gw.Recorded()
	require.Len(t, recs, 1, "enlist must issue exactly one request")
	rec := recs[0]
	require.Equal(t, http.MethodPost, rec.Method)
	require.Equal(t, "/v1/listings", rec.Path)

	// The create is signed with the party key: the bearer token verifies
	// offline against the party JWKS.
	require.True(t, strings.HasPrefix(rec.Authorization, "Bearer "),
		"enlist must carry a bearer ks-party+jwt")
	tok := strings.TrimSpace(strings.TrimPrefix(rec.Authorization, "Bearer "))
	jwks, err := gatewaytest.Ed25519JWKS(currentKey.Public().(ed25519.PublicKey))
	require.NoError(t, err)
	srv, err := gatewaytest.NewJWKSServer(jwks)
	require.NoError(t, err)
	t.Cleanup(srv.Close)
	_, err = srv.Verify(tok)
	require.NoError(t, err, "enlist must be signed with the party key")

	// The body carries every listing field passed on the command line.
	var body struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		EndpointURL string `json:"endpoint_url"`
		Price       int64  `json:"price"`
		Invocation  string `json:"invocation"`
		PriceScope  string `json:"price_scope"`
	}
	require.NoError(t, json.Unmarshal(rec.Body, &body))
	assert.Equal(t, "send-magiclink", body.Name, "name must be submitted")
	assert.Equal(t, "Deliver a magic login link", body.Description,
		"description must be submitted")
	assert.Equal(t, "https://n8n.example.com/webhook/send-magiclink",
		body.EndpointURL, "endpoint must be submitted")
	assert.Equal(t, int64(53200), body.Price, "price must be submitted")
	assert.Equal(t, "webhook", body.Invocation, "invocation must be submitted")
	assert.Equal(t, "party", body.PriceScope, "price-scope must be submitted")
}

// TestEnlistDuplicateNameExitsNonZeroWithGatewayMessage covers the T47
// contract assertion: a duplicate listing name makes enlist exit non-zero
// with the gateway message.
// Parameters: t is the test harness.
// Returns: nothing; fails on any deviation.
func TestEnlistDuplicateNameExitsNonZeroWithGatewayMessage(t *testing.T) {
	gw := gatewaytest.NewGateway()
	t.Cleanup(gw.Close)
	gw.Script(http.MethodPost, "/v1/listings", gatewaytest.ScriptStep{
		Status: http.StatusConflict,
		Body: []byte(`{"error":{"code":"GW-LISTING-009",` +
			`"message":"listing name already exists","request_id":"req-listing-009"}}`),
	})
	enlistEnv(t, gw)

	var stdout, stderr bytes.Buffer
	err := run([]string{
		"enlist",
		"--name", "send-magiclink",
		"--endpoint", "https://n8n.example.com/webhook/send-magiclink",
	}, &stdout, &stderr)
	require.Error(t, err,
		"a duplicate listing name must fail the command (non-zero exit)")
	require.Empty(t, stdout.String(),
		"a rejected enlist must print no listing_id")
	require.Empty(t, stderr.String(),
		"a rejected enlist must not write to stderr")

	// The failure surfaces as the mapped gateway error carrying the
	// gateway's message and its GW-* code.
	var mapped *exceptions.CustomError
	require.ErrorAs(t, err, &mapped, "enlist must return the mapped gateway error")
	assert.Contains(t, mapped.ClientMessage, "listing name already exists",
		"the mapped error must carry the gateway message")
	assert.Contains(t, err.Error(), "GW-LISTING-009",
		"the mapped error must carry the gateway rejection code")
}
