package gateway

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/konsulin-care/konsulin-api/internal/app/config"
	"github.com/konsulin-care/konsulin-api/internal/app/contracts"

	"github.com/konsulin-care/gateway-sdk-go/gatewaytest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newPaymentTestAdapter builds an enabled adapter against rawURL under env
// "test" (its loopback http:// URL requires WithInsecureTransport) with a
// deterministic Ed25519 seed byte i = i.
// Parameters: t is the test harness, rawURL the fake gateway base URL.
// Returns: the adapter; fails the test on error.
func newPaymentTestAdapter(t *testing.T, rawURL string) *Client {
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

// TestCreateSellerPaymentSendsSellerAmountWithIdempotencyKey covers T26
// assertion 1: CreateSellerPayment posts domain, opaque, amount and both
// return URLs to POST /v1/payments carrying an Idempotency-Key and returns
// the gateway's payment_id and payment_url.
// Parameters: t is the test harness.
// Returns: nothing; fails on any deviation.
func TestCreateSellerPaymentSendsSellerAmountWithIdempotencyKey(t *testing.T) {
	gw := gatewaytest.NewGateway()
	t.Cleanup(gw.Close)
	c := newPaymentTestAdapter(t, gw.URL())

	paymentID, paymentURL, err := c.CreateSellerPayment(context.Background(),
		"seller.example.com", "opp-9001", 15000, "cust-ref-abc",
		"https://app.example/pay/success", "https://app.example/pay/cancel")
	require.NoError(t, err, "the fake gateway's create must be accepted")
	assert.Equal(t, "01HZYPAYMENT000001", paymentID, "must return the gateway payment_id")
	assert.Equal(t, "https://checkout.example/pay-01hzypayment000001", paymentURL,
		"must return the gateway payment_url")

	recs := gw.Recorded()
	require.Len(t, recs, 1, "one create must issue exactly one request")
	require.Equal(t, http.MethodPost, recs[0].Method)
	require.Equal(t, "/v1/payments", recs[0].Path)
	require.NotEmpty(t, recs[0].IdempotencyKey,
		"a logical create must carry an Idempotency-Key")

	var body map[string]any
	require.NoError(t, json.Unmarshal(recs[0].Body, &body), "body must be JSON")
	assert.Equal(t, "seller.example.com", body["domain"], "must send domain")
	assert.Equal(t, "opp-9001", body["opaque"], "must send opaque")
	assert.Equal(t, float64(15000), body["amount"], "must send amount")
	assert.Equal(t, "cust-ref-abc", body["customer_ref"], "must send customer_ref")
	assert.Equal(t, "https://app.example/pay/success", body["success_return_url"],
		"must send the success return URL")
	assert.Equal(t, "https://app.example/pay/cancel", body["cancel_return_url"],
		"must send the cancel return URL")
	_, hasEmail := body["customer_email"]
	assert.False(t, hasEmail, "must never send customer_email")
	_, hasName := body["customer_name"]
	assert.False(t, hasName, "must never send customer_name")
}

// TestCreateListingPaymentSendsListingRefWithoutCustomerPII covers T26
// assertion 2: CreateListingPayment posts listing_id and customer_ref to
// POST /v1/payments with no customer email or name anywhere in the body.
// Parameters: t is the test harness.
// Returns: nothing; fails on any deviation.
func TestCreateListingPaymentSendsListingRefWithoutCustomerPII(t *testing.T) {
	gw := gatewaytest.NewGateway()
	t.Cleanup(gw.Close)
	c := newPaymentTestAdapter(t, gw.URL())

	paymentID, paymentURL, err := c.CreateListingPayment(context.Background(),
		"01HZYLISTING000001", "cust-ref-abc")
	require.NoError(t, err, "the fake gateway's create must be accepted")
	assert.Equal(t, "01HZYPAYMENT000001", paymentID, "must return the gateway payment_id")
	assert.Equal(t, "https://checkout.example/pay-01hzypayment000001", paymentURL,
		"must return the gateway payment_url")

	recs := gw.Recorded()
	require.Len(t, recs, 1, "one create must issue exactly one request")
	require.Equal(t, http.MethodPost, recs[0].Method)
	require.Equal(t, "/v1/payments", recs[0].Path)
	require.NotEmpty(t, recs[0].IdempotencyKey,
		"a logical create must carry an Idempotency-Key")

	var body map[string]any
	require.NoError(t, json.Unmarshal(recs[0].Body, &body), "body must be JSON")
	assert.Equal(t, "01HZYLISTING000001", body["listing_id"], "must send listing_id")
	assert.Equal(t, "cust-ref-abc", body["customer_ref"], "must send customer_ref")
	_, hasEmail := body["customer_email"]
	assert.False(t, hasEmail, "must never send customer_email")
	_, hasName := body["customer_name"]
	assert.False(t, hasName, "must never send customer_name")
}

// TestGetPaymentReturnsGatewayStatus covers T26 assertion 3 (first half):
// GetPayment issues GET /v1/payments/{id} — a read, so no Idempotency-Key —
// and returns the gateway's status.
// Parameters: t is the test harness.
// Returns: nothing; fails on any deviation.
func TestGetPaymentReturnsGatewayStatus(t *testing.T) {
	gw := gatewaytest.NewGateway()
	t.Cleanup(gw.Close)
	c := newPaymentTestAdapter(t, gw.URL())

	status, err := c.GetPayment(context.Background(), "01HZYPAYMENT000001")
	require.NoError(t, err, "the fake gateway's read must be accepted")
	assert.Equal(t, "PAID", status, "must return the gateway status")

	recs := gw.Recorded()
	require.Len(t, recs, 1, "one read must issue exactly one request")
	require.Equal(t, http.MethodGet, recs[0].Method)
	require.Equal(t, "/v1/payments/01HZYPAYMENT000001", recs[0].Path)
	assert.Empty(t, recs[0].IdempotencyKey, "a read must not send an Idempotency-Key")
}

// TestPaymentGatewayErrorsMappedByMapError covers T26 assertion 3 (second
// half): a gateway failure surfaced by the payment methods renders through
// MapError keeping the gateway's HTTP status, GW-* code and request_id, and
// a disabled adapter maps to 503 — errors always go through MapError.
// Parameters: t is the test harness.
// Returns: nothing; fails on any deviation.
func TestPaymentGatewayErrorsMappedByMapError(t *testing.T) {
	t.Run("gateway error envelope", func(t *testing.T) {
		gw := gatewaytest.NewGateway()
		t.Cleanup(gw.Close)
		gw.Script(http.MethodGet, "/v1/payments/{id}", gatewaytest.ScriptStep{
			Status: http.StatusNotFound,
			Body: []byte(`{"error":{"code":"GW-PAY-004",` +
				`"message":"payment not found","request_id":"gw-req-42"}}`),
		})
		c := newPaymentTestAdapter(t, gw.URL())

		_, err := c.GetPayment(context.Background(), "pay-missing")
		require.Error(t, err, "a gateway failure must surface as an error")
		ce := MapError(err)
		require.NotNil(t, ce, "payment errors must map through MapError")
		assert.Equal(t, http.StatusNotFound, ce.StatusCode,
			"the gateway HTTP status must be kept")
		assert.Contains(t, ce.DevMessage, "GW-PAY-004",
			"dev_message must carry the GW-* code")
		assert.Contains(t, ce.DevMessage, "gw-req-42",
			"dev_message must carry the gateway request_id")
	})

	t.Run("disabled adapter", func(t *testing.T) {
		c, err := New(config.AppGateway{}, "test", nil)
		require.NoError(t, err)
		_, _, err = c.CreateSellerPayment(context.Background(),
			"seller.example.com", "opp-1", 1000, "cust", "", "")
		require.ErrorIs(t, err, contracts.ErrGatewayNotConfigured,
			"a disabled adapter must fail closed before any I/O")
		ce := MapError(err)
		require.NotNil(t, ce, "the failure must map through MapError")
		assert.Equal(t, http.StatusServiceUnavailable, ce.StatusCode,
			"a missing gateway must answer 503")
	})
}
