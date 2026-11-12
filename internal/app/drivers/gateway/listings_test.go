package gateway

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"

	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/contracts"

	"github.com/konsulin-care/gateway-sdk-go/wire"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// listingTestProvider and listingTestName identify the listing under test.
const (
	listingTestProvider = "01HZYPROVIDER00000042"
	listingTestName     = "research-access"
)

// listingCall is one request observed by the listings fake.
type listingCall struct {
	Method         string
	Path           string
	Query          url.Values
	Body           []byte
	IdempotencyKey string
}

// listingsFake serves GET and POST /v1/listings with recorded requests:
// every read answers the configured rows (so a cache miss falls back to
// the same fixture), every create answers one canned listing id.
type listingsFake struct {
	mu    sync.Mutex
	calls []listingCall
	rows  []wire.ListingRow
}

// newListingsFake starts the listings fake gateway surface.
// Parameters: t is the test harness, rows the active listings served on
// every GET /v1/listings.
// Returns: the fake server URL base and the recorder; the server closes
// with the test.
func newListingsFake(t *testing.T, rows []wire.ListingRow) (*httptest.Server, *listingsFake) {
	t.Helper()
	fake := &listingsFake{rows: rows}
	srv := httptest.NewServer(http.HandlerFunc(fake.serve))
	t.Cleanup(srv.Close)
	return srv, fake
}

// serve records one request and answers the listings routes.
// Parameters: w the response writer, r the inbound request.
// Returns: nothing; an unmatched route answers 404.
func (f *listingsFake) serve(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	f.mu.Lock()
	f.calls = append(f.calls, listingCall{
		Method: r.Method, Path: r.URL.Path, Query: r.URL.Query(),
		Body: body, IdempotencyKey: r.Header.Get("Idempotency-Key"),
	})
	rows := f.rows
	f.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/v1/listings":
		_ = json.NewEncoder(w).Encode(wire.GetListingsResponse{Listings: rows})
	case r.Method == http.MethodPost && r.URL.Path == "/v1/listings":
		_, _ = w.Write([]byte(`{"listing_id":"01HZYLISTING000001","state":"active"}`))
	default:
		http.NotFound(w, r)
	}
}

// recorded returns every observed call in arrival order.
// Parameters: none.
// Returns: a copy of the call log.
func (f *listingsFake) recorded() []listingCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]listingCall, len(f.calls))
	copy(out, f.calls)
	return out
}

// reads returns the observed GET /v1/listings calls in arrival order.
// Parameters: none.
// Returns: the listing read log.
func (f *listingsFake) reads() []listingCall {
	var out []listingCall
	for _, call := range f.recorded() {
		if call.Method == http.MethodGet && call.Path == "/v1/listings" {
			out = append(out, call)
		}
	}
	return out
}

// creates returns the observed POST /v1/listings calls in arrival order.
// Parameters: none.
// Returns: the listing create log.
func (f *listingsFake) creates() []listingCall {
	var out []listingCall
	for _, call := range f.recorded() {
		if call.Method == http.MethodPost && call.Path == "/v1/listings" {
			out = append(out, call)
		}
	}
	return out
}

// newListingTestAdapter builds an enabled adapter against rawURL under env
// "test" (its loopback http:// URL requires WithInsecureTransport) with a
// deterministic Ed25519 seed.
// Parameters: t is the test harness, rawURL the fake gateway base URL.
// Returns: the adapter; fails the test on error.
func newListingTestAdapter(t *testing.T, rawURL string) *Client {
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

// listingWebhookRow is the active webhook listing served under test.
// Parameters: none.
// Returns: the webhook listing row.
func listingWebhookRow() wire.ListingRow {
	return wire.ListingRow{
		ListingID: "01HZYLISTING000001", ProviderPartyID: listingTestProvider,
		Name: listingTestName, Description: "Sealed webhook invocation",
		Price: 0, Currency: "IDR", Invocation: "webhook",
		PriceScope: "user", Metering: "gateway",
	}
}

// TestFindListingReturnsWebhookListingServedFromCache covers T30
// assertion 1: FindListing resolves the active webhook listing through the
// SDK FindByName filters and serves repeated lookups from the 60s TTL
// cache — one gateway read for many lookups.
// Parameters: t is the test harness.
// Returns: nothing; fails on any deviation.
func TestFindListingReturnsWebhookListingServedFromCache(t *testing.T) {
	srv, fake := newListingsFake(t, []wire.ListingRow{listingWebhookRow()})
	c := newListingTestAdapter(t, srv.URL)
	ctx := context.Background()

	first, err := c.FindListing(ctx, listingTestProvider, listingTestName)
	require.NoError(t, err, "the active listing must resolve")
	require.NotNil(t, first)
	assert.Equal(t, "01HZYLISTING000001", first.ListingID, "must return the listing id")
	assert.Equal(t, "webhook", first.Invocation, "must return the webhook listing")
	assert.Equal(t, listingTestProvider, first.ProviderPartyID, "must return the provider")
	assert.Equal(t, "user", first.PriceScope, "must return the price scope")

	for i := 0; i < 2; i++ {
		again, err := c.FindListing(ctx, listingTestProvider, listingTestName)
		require.NoError(t, err, "repeated lookups must keep resolving")
		require.NotNil(t, again)
		assert.Equal(t, *first, *again, "cached lookups must return the same row")
	}

	reads := fake.reads()
	require.Len(t, reads, 1,
		"repeated lookups within the TTL must be served from the cache")
	assert.Equal(t, listingTestProvider, reads[0].Query.Get("provider_party_id"),
		"FindByName must filter by provider_party_id")
	assert.Equal(t, listingTestName, reads[0].Query.Get("name"),
		"FindByName must filter by name")
}

// TestFindListingUnknownTriggersOneRefetchThenNotFound covers T30
// assertion 2: an unknown listing refetches exactly once and then fails
// closed with contracts.ErrListingNotFound.
// Parameters: t is the test harness.
// Returns: nothing; fails on any deviation.
func TestFindListingUnknownTriggersOneRefetchThenNotFound(t *testing.T) {
	srv, fake := newListingsFake(t, nil)
	c := newListingTestAdapter(t, srv.URL)

	_, err := c.FindListing(context.Background(), listingTestProvider, "ghost-listing")
	require.Error(t, err, "an unknown listing must fail")
	assert.ErrorIs(t, err, contracts.ErrListingNotFound,
		"a persistent miss must surface ErrListingNotFound")

	reads := fake.reads()
	require.Len(t, reads, 2, "a miss must trigger exactly one refetch")
	for i, read := range reads {
		assert.Equal(t, listingTestProvider, read.Query.Get("provider_party_id"),
			"read %d must filter by provider_party_id", i)
		assert.Equal(t, "ghost-listing", read.Query.Get("name"),
			"read %d must filter by name", i)
	}
}

// TestListWebhookListingsReturnsCachedWebhookListings covers T30
// assertion 3: ListWebhookListings serves the active webhook listings
// (invocation=webhook) and caches the discovery read.
// Parameters: t is the test harness.
// Returns: nothing; fails on any deviation.
func TestListWebhookListingsReturnsCachedWebhookListings(t *testing.T) {
	commodity := wire.ListingRow{
		ListingID: "01HZYLISTING000002", ProviderPartyID: listingTestProvider,
		Name: "dataset-extract", Price: 53200, Currency: "IDR",
		Invocation: "commodity", PriceScope: "party", Metering: "gateway",
	}
	srv, fake := newListingsFake(t, []wire.ListingRow{listingWebhookRow(), commodity})
	c := newListingTestAdapter(t, srv.URL)
	ctx := context.Background()

	first, err := c.ListWebhookListings(ctx)
	require.NoError(t, err, "the webhook discovery read must succeed")
	require.Len(t, first, 1, "only webhook listings may be returned")
	assert.Equal(t, "01HZYLISTING000001", first[0].ListingID, "must return the webhook listing")
	assert.Equal(t, "webhook", first[0].Invocation, "must return the webhook listing")

	second, err := c.ListWebhookListings(ctx)
	require.NoError(t, err, "repeated discovery reads must keep resolving")
	assert.Equal(t, first, second, "repeated reads must return the cached rows")

	reads := fake.reads()
	require.Len(t, reads, 1, "the discovery read must be cached")
	assert.Equal(t, "webhook", reads[0].Query.Get("invocation"),
		"the read must filter by invocation=webhook")
}

// TestEnlistPostsListingThroughSDK covers T30's Enlist contract: Enlist
// posts the listing to POST /v1/listings carrying an Idempotency-Key and
// returns the gateway's listing id.
// Parameters: t is the test harness.
// Returns: nothing; fails on any deviation.
func TestEnlistPostsListingThroughSDK(t *testing.T) {
	srv, fake := newListingsFake(t, nil)
	c := newListingTestAdapter(t, srv.URL)

	listingID, err := c.Enlist(context.Background(), contracts.EnlistRequest{
		Name:        listingTestName,
		Description: "Sealed webhook invocation",
		EndpointURL: "https://provider.example.com/hook",
		Price:       15000,
		Invocation:  "webhook",
		PriceScope:  "user",
		Metering:    "gateway",
	})
	require.NoError(t, err, "the fake gateway's create must be accepted")
	assert.Equal(t, "01HZYLISTING000001", listingID, "must return the created listing id")

	creates := fake.creates()
	require.Len(t, creates, 1, "one enlist must issue exactly one create")
	require.NotEmpty(t, creates[0].IdempotencyKey,
		"a logical create must carry an Idempotency-Key")

	var body map[string]any
	require.NoError(t, json.Unmarshal(creates[0].Body, &body), "body must be JSON")
	assert.Equal(t, listingTestName, body["name"], "must send name")
	assert.Equal(t, "Sealed webhook invocation", body["description"], "must send description")
	assert.Equal(t, "https://provider.example.com/hook", body["endpoint_url"],
		"must send endpoint_url")
	assert.Equal(t, float64(15000), body["price"], "must send price")
	assert.Equal(t, "webhook", body["invocation"], "must send invocation")
	assert.Equal(t, "user", body["price_scope"], "must send price_scope")
	assert.Equal(t, "gateway", body["metering"], "must send metering")
}
