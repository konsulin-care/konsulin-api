package gateway

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/konsulin-care/konsulin-api/internal/app/contracts"

	"github.com/konsulin-care/gateway-sdk-go/listings"
	"github.com/konsulin-care/gateway-sdk-go/wire"
)

// listingGuard proves at compile time that the adapter satisfies
// contracts.ListingDirectory — the interface and the implementation must
// land together so the package compiles at every commit.
var _ contracts.ListingDirectory = (*Client)(nil)

// listingCacheTTL is how long one listing lookup (a FindByName resolution
// or the webhook discovery read) is served from the adapter's memo before
// the gateway is read again.
const listingCacheTTL = 60 * time.Second

// listingKey identifies one FindListing cache entry.
type listingKey struct {
	provider string
	name     string
}

// listingCacheEntry is one cached FindListing row and its fetch time.
type listingCacheEntry struct {
	row       contracts.Listing
	fetchedAt time.Time
}

// listingWebhookEntry is the cached webhook discovery read.
type listingWebhookEntry struct {
	rows      []contracts.Listing
	fetchedAt time.Time
}

// listingCache is one adapter's listing memo: FindListing rows keyed by
// provider+name plus the webhook discovery list, each valid for
// listingCacheTTL. It lives outside the Client struct (owned by client.go)
// and is held per adapter in listingCaches, so two adapters never share
// entries.
type listingCache struct {
	mu       sync.Mutex
	found    map[listingKey]listingCacheEntry
	webhooks *listingWebhookEntry
}

// listingCaches holds one listingCache per adapter, created on first use.
var listingCaches sync.Map // *Client -> *listingCache

// listingCacheFor returns the adapter's listing memo, creating it on first
// use.
// Parameters: c is the adapter.
// Returns: the adapter's listingCache; never nil. Never panics.
func listingCacheFor(c *Client) *listingCache {
	if v, ok := listingCaches.Load(c); ok {
		return v.(*listingCache)
	}
	v, _ := listingCaches.LoadOrStore(c,
		&listingCache{found: make(map[listingKey]listingCacheEntry)})
	return v.(*listingCache)
}

// find returns the cached row for key when it is younger than
// listingCacheTTL.
// Parameters: key is the provider+name to look up.
// Returns: the cached listing and true within the TTL, else false.
func (lc *listingCache) find(key listingKey) (contracts.Listing, bool) {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	entry, ok := lc.found[key]
	if !ok || time.Since(entry.fetchedAt) >= listingCacheTTL {
		return contracts.Listing{}, false
	}
	return entry.row, true
}

// put memoizes one resolved FindListing row.
// Parameters: key is the provider+name, row the resolved listing.
// Returns: nothing.
func (lc *listingCache) put(key listingKey, row contracts.Listing) {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	lc.found[key] = listingCacheEntry{row: row, fetchedAt: time.Now()}
}

// webhook returns the cached webhook discovery rows when younger than
// listingCacheTTL.
// Parameters: none.
// Returns: a copy of the cached rows and true within the TTL, else false.
func (lc *listingCache) webhook() ([]contracts.Listing, bool) {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	if lc.webhooks == nil || time.Since(lc.webhooks.fetchedAt) >= listingCacheTTL {
		return nil, false
	}
	return append([]contracts.Listing(nil), lc.webhooks.rows...), true
}

// putWebhook memoizes one webhook discovery read.
// Parameters: rows the resolved webhook listings.
// Returns: nothing.
func (lc *listingCache) putWebhook(rows []contracts.Listing) {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	lc.webhooks = &listingWebhookEntry{
		rows:      append([]contracts.Listing(nil), rows...),
		fetchedAt: time.Now(),
	}
}

// invalidate drops every cached lookup for name (any provider) and the
// webhook discovery read — a re-enlist changes what the next read serves
// (BK-MARKET-C01).
// Parameters: name is the enlisted listing title.
// Returns: nothing.
func (lc *listingCache) invalidate(name string) {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	for key := range lc.found {
		if key.name == name {
			delete(lc.found, key)
		}
	}
	lc.webhooks = nil
}

// listingRoutes binds the SDK listing routes (SDK-API-001) to the adapter's
// SDK client. It fails closed: a disabled adapter reports
// contracts.ErrGatewayNotConfigured before any I/O.
// Parameters: none — reads the adapter's own state.
// Returns: the SDK listing routes, or the error explaining why they cannot
// be built. Never panics.
func (c *Client) listingRoutes() (*listings.Client, error) {
	if !c.enabled {
		return nil, contracts.ErrGatewayNotConfigured
	}
	return listings.New(c.sdk)
}

// fromWireListing converts one SDK read row into the contracts listing.
// Parameters: row is the decoded gateway row.
// Returns: the backend-side mirror of the row. Never panics.
func fromWireListing(row wire.ListingRow) contracts.Listing {
	return contracts.Listing{
		ListingID: row.ListingID, ProviderPartyID: row.ProviderPartyID,
		Name: row.Name, Description: row.Description, EndpointURL: row.EndpointURL,
		Price: row.Price, Currency: row.Currency, Invocation: row.Invocation,
		PriceScope: row.PriceScope, Metering: row.Metering,
		ProviderTier: row.ProviderTier, ProviderOrgName: row.ProviderOrgName,
	}
}

// isListingMiss reports whether err is the SDK FindByName zero-match
// failure. The SDK formats that miss with fmt.Errorf and no wrapped
// sentinel (pinned in go.mod), so the classification keys on its stable
// message; a multiple-match failure reads differently and stays raw.
// Parameters: err is the FindByName error.
// Returns: whether the gateway reported no matching listing. Never panics.
func isListingMiss(err error) bool {
	return err != nil && strings.Contains(err.Error(), "no listing named")
}

// FindListing resolves the single active listing named name provided by
// provider through the SDK listings FindByName route: a cached row within
// listingCacheTTL is served without I/O; on a miss exactly one refetch runs
// (covering gateway propagation after an enlist) before the lookup fails
// closed with contracts.ErrListingNotFound. A disabled adapter reports
// contracts.ErrGatewayNotConfigured before any I/O. Implements
// contracts.ListingDirectory.
// Parameters: ctx bounds the lookup, provider the providing party, name the
// listing title.
// Returns: the active listing, or the error — ErrListingNotFound on a
// persistent miss. Never panics.
func (c *Client) FindListing(ctx context.Context, provider, name string) (*contracts.Listing, error) {
	routes, err := c.listingRoutes()
	if err != nil {
		return nil, fmt.Errorf("gateway: find listing %q: %w", name, err)
	}
	cache := listingCacheFor(c)
	key := listingKey{provider: provider, name: name}
	if row, ok := cache.find(key); ok {
		return &row, nil
	}
	row, err := routes.FindByName(ctx, provider, name)
	if err != nil {
		// One refetch on miss: a just-enlisted row may not be visible yet.
		row, err = routes.FindByName(ctx, provider, name)
		if err != nil {
			if isListingMiss(err) {
				return nil, fmt.Errorf("gateway: find listing %q: %w",
					name, contracts.ErrListingNotFound)
			}
			return nil, fmt.Errorf("gateway: find listing %q: %w", name, err)
		}
	}
	out := fromWireListing(*row)
	cache.put(key, out)
	return &out, nil
}

// ListWebhookListings reads the active invocation=webhook listings at
// GET /v1/listings through the SDK listing routes — a read, so it sends no
// Idempotency-Key — keeps only webhook rows and serves repeated reads from
// its listingCacheTTL cache. A disabled adapter reports
// contracts.ErrGatewayNotConfigured before any I/O. Implements
// contracts.ListingDirectory.
// Parameters: ctx bounds the read.
// Returns: the active webhook listings, plus the error to render through
// MapError. Never panics.
func (c *Client) ListWebhookListings(ctx context.Context) ([]contracts.Listing, error) {
	routes, err := c.listingRoutes()
	if err != nil {
		return nil, fmt.Errorf("gateway: list webhook listings: %w", err)
	}
	cache := listingCacheFor(c)
	if rows, ok := cache.webhook(); ok {
		return rows, nil
	}
	resp, err := routes.List(ctx, listings.WithInvocation("webhook"))
	if err != nil {
		return nil, fmt.Errorf("gateway: list webhook listings: %w", err)
	}
	rows := make([]contracts.Listing, 0, len(resp.Listings))
	for _, listed := range resp.Listings {
		if listed.Invocation != "webhook" {
			continue
		}
		rows = append(rows, fromWireListing(listed))
	}
	cache.putWebhook(rows)
	return append([]contracts.Listing(nil), rows...), nil
}

// Enlist creates or re-enlists one listing at POST /v1/listings through the
// SDK listings Create route — party-signed and idempotent (one
// Idempotency-Key per logical create, SDK-CLIENT-002). Any cached lookup for
// the enlisted name is dropped so the next read sees the new listing
// (BK-MARKET-C01). A disabled adapter reports
// contracts.ErrGatewayNotConfigured before any I/O. Implements
// contracts.ListingDirectory.
// Parameters: ctx bounds the call, req the listing to enlist.
// Returns: the created listing id, plus the error to render through
// MapError. Never panics.
func (c *Client) Enlist(ctx context.Context, req contracts.EnlistRequest) (string, error) {
	routes, err := c.listingRoutes()
	if err != nil {
		return "", fmt.Errorf("gateway: enlist listing %q: %w", req.Name, err)
	}
	out, err := routes.Create(ctx, wire.CreateListingRequest{
		Name:        req.Name,
		Description: req.Description,
		EndpointURL: req.EndpointURL,
		Price:       req.Price,
		Invocation:  req.Invocation,
		PriceScope:  req.PriceScope,
		Metering:    req.Metering,
	})
	if err != nil {
		return "", fmt.Errorf("gateway: enlist listing %q: %w", req.Name, err)
	}
	listingCacheFor(c).invalidate(req.Name)
	return out.ListingID, nil
}
