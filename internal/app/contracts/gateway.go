package contracts

import (
	"context"
	"errors"
)

// ErrGatewayNotConfigured indicates a gateway client or dependency is missing
// from the deployment configuration. Callers should surface it as a
// configuration problem, not a client error.
var ErrGatewayNotConfigured = errors.New("gateway not configured")

// ErrGatewayNotEnrolled indicates the authenticated subject has not completed
// gateway enrollment. Callers should surface it as an onboarding requirement.
var ErrGatewayNotEnrolled = errors.New("gateway not enrolled")

// ErrListingNotFound indicates the gateway served no active listing for the
// requested provider and name after the adapter's single refetch. Callers
// should surface it as a missing marketplace entry, not a transport failure.
var ErrListingNotFound = errors.New("listing not found")

// GatewayStatus reports the availability of a gateway integration.
type GatewayStatus interface {
	// Enabled reports whether the gateway integration is configured and active.
	Enabled() bool
	// Enrolled reports whether the current subject is enrolled with the gateway.
	Enrolled() bool
}

// PartyRegistry exposes the gateway party's enrollment lifecycle as the
// gateway itself reports it (BK-PARTYID-002): the gateway is the only
// enrollment source of truth, never a local flag.
type PartyRegistry interface {
	// IsEnrolled fetches GET /v1/parties/{id}/jwks for the adapter's own
	// party and reports whether the served JWKS contains the adapter's own
	// kid (RFC 7638 thumbprint). It fails closed: a missing configuration
	// reports ErrGatewayNotConfigured; transport or decode failures return
	// false with the error; a JWKS without the kid returns (false, nil).
	// Parameters: ctx bounds the JWKS fetch.
	// Returns: whether the party JWKS serves the adapter's own kid, plus the
	// error explaining a failed probe.
	IsEnrolled(ctx context.Context) (bool, error)
	// Enroll completes first-startup enrollment (BK-PARTYID-002): it
	// submits the single-use enrollment token, the adapter's signing
	// public key and the enc_public_key derived from
	// APP_PARTY_ENC_PRIVATE_KEY through the SDK parties.Enroll route
	// (POST /v1/parties/{id}/enroll). It fails closed: a disabled adapter
	// or missing key material reports ErrGatewayNotConfigured before any
	// I/O.
	// Parameters: ctx bounds the enrollment call, token is the single-use
	// enrollment token delivered out of band by the gateway admin.
	// Returns: nil once the gateway accepts the enrollment, or the error
	// explaining the failure. Never logs key values.
	Enroll(ctx context.Context, token string) error
	// Rotate performs one self-service key rotation (BK-PARTYID-003): it
	// submits the next signing key's public key and derived kid (RFC 7638
	// thumbprint) to the SDK parties.Rotate route
	// (POST /v1/parties/{id}/keys), signed with the CURRENT key, then
	// switches the adapter signer to the next key so subsequent requests
	// carry the next kid. It fails closed: a disabled adapter or missing or
	// unparseable next key material reports ErrGatewayNotConfigured (or a
	// parse error naming only the variable) before any I/O.
	// Parameters: ctx bounds the rotation call, nextSig is the next signing
	// key value (APP_PARTY_NEXT_PRIVATE_KEY encoding), nextEnc the next
	// X25519 key value (APP_PARTY_NEXT_ENC_PRIVATE_KEY encoding).
	// Returns: nil once the gateway accepts the rotation and the adapter
	// signer has switched, or the error explaining the failure. Never logs
	// key values.
	Rotate(ctx context.Context, nextSig, nextEnc string) error
}

// PaymentClient routes the gateway payment endpoints (SDK-API-001): the
// seller-amount create, the listing create and the reconciliation read.
// Every create is one logical create: it carries a single Idempotency-Key
// the SDK generates once and holds stable for that create (SDK-CLIENT-002).
// The request body never carries customer PII — no customer_email and no
// customer_name; the end user reaches the gateway only as a peppered
// customer_ref. Errors surface raw and are rendered through
// gateway.MapError.
type PaymentClient interface {
	// CreateSellerPayment initiates a seller-amount payment
	// (POST /v1/payments in seller-amount mode: domain + opaque +
	// amount >= 1, return URLs optional).
	// Parameters: ctx bounds the call, domain the seller-mode domain,
	// opaque the seller-mode opaque identifier, amount the amount in IDR
	// minor units, customerRef the peppered end-user hash (never
	// plaintext PII), successURL and cancelURL the buyer redirect targets
	// (empty sends no return URL).
	// Returns: the created payment_id and its payment_url ("" when the
	// gateway settles immediately and serves a null URL), plus the error
	// to render through gateway.MapError. Never panics.
	CreateSellerPayment(ctx context.Context, domain, opaque string, amount int64,
		customerRef, successURL, cancelURL string) (paymentID, paymentURL string, err error)
	// CreateListingPayment initiates a listing-mode payment
	// (POST /v1/payments in listing mode: the gateway reads the price
	// from the listing row).
	// Parameters: ctx bounds the call, listingID the active listing to
	// purchase, customerRef the peppered end-user hash required for
	// price_scope=user listings (never plaintext PII).
	// Returns: the created payment_id and its payment_url ("" when the
	// gateway settles immediately and serves a null URL), plus the error
	// to render through gateway.MapError. Never panics.
	CreateListingPayment(ctx context.Context, listingID, customerRef string) (paymentID, paymentURL string, err error)
	// GetPayment fetches one payment's reconciliation read
	// (GET /v1/payments/{id}): a read, so it sends no Idempotency-Key.
	// Parameters: ctx bounds the call, paymentID the payment to fetch.
	// Returns: the gateway status ("PENDING", "PAID", "EXPIRED" or
	// "CANCELED"), plus the error to render through gateway.MapError.
	// Never panics.
	GetPayment(ctx context.Context, paymentID string) (status string, err error)
}

// WebhookClient routes the gateway webhook endpoints (SDK-API-001). Every
// invocation payload is sealed to the recipient's use:enc key through the
// SDK bound envelope (envelope.SealBound, bound from/to/service) before it
// leaves the adapter: plaintext payload bytes are never sent. An
// application/jose reply is opened with envelope.OpenBound under swapped
// expectations. A missing recipient use:enc key fails closed before any
// webhook request is sent. Errors surface raw and are rendered through
// gateway.MapError.
type WebhookClient interface {
	// Enqueue queues one async invocation at POST /v1/webhooks: the body
	// carries to, service, user_ref and payload — the payload is the
	// compact JWE as a JSON string, sealed to the recipient's use:enc
	// key and bound from the adapter's party, to the recipient and to
	// the service. The call is idempotent, so it carries one
	// Idempotency-Key (SDK-CLIENT-002).
	// Parameters: ctx bounds the call, to the recipient party, service
	// the listing name, userRef the peppered end-user hash ("" when the
	// listing is not user-scoped), data the plaintext payload to seal.
	// Returns: nil once the gateway accepts the enqueue (202), or the
	// error — including a recipient-key resolution failure raised before
	// any webhook request — to render through gateway.MapError.
	Enqueue(ctx context.Context, to, service, userRef string, data []byte) error
	// Sync forwards one invocation inline at POST /v1/webhooks/sync and
	// opens the reply: the request body is the same sealed payload as
	// Enqueue, and an application/jose reply is opened with
	// envelope.OpenBound under the swapped expectations (from = the
	// recipient, to = the adapter's party, service = the service) using
	// APP_PARTY_ENC_PRIVATE_KEY.
	// Parameters: ctx bounds the call, to the recipient party, service
	// the listing name, userRef the peppered end-user hash ("" when the
	// listing is not user-scoped), data the plaintext payload to seal.
	// Returns: the opened reply data and nil, or nil and the error — a
	// non-jose reply, an unopenable envelope or a recipient-key
	// resolution failure raised before any webhook request — to render
	// through gateway.MapError.
	Sync(ctx context.Context, to, service, userRef string, data []byte) ([]byte, error)
	// Status reads one enqueued invocation's delivery state at GET
	// /v1/webhooks/{id}/status — a read, so it sends no Idempotency-Key.
	// Parameters: ctx bounds the call, id the enqueued webhook id.
	// Returns: the gateway's state ("pending", "delivered" or
	// "failed"), plus the error to render through gateway.MapError.
	Status(ctx context.Context, id string) (state string, err error)
}

// IdentityCandidate is one resolve candidate as the adapter surfaces it to
// contracts consumers — the backend-side mirror of the gateway row; the
// SDK's own wire types never leave the gateway driver package.
type IdentityCandidate struct {
	// GlobalKey is the shared identity key behind the candidate.
	GlobalKey string
	// PartyID is the party holding the linked resource.
	PartyID string
	// LocalResourceID is that party's local resource reference.
	LocalResourceID string
	// MatchField is "email", "phone", "email+phone" or "link".
	MatchField string
}

// IdentityClient routes the gateway identity-plane endpoints (SDK-API-001,
// BK-FED-003): register, resolve and link through the SDK identities
// routes. Every request body carries hashed identity fields only — the
// peppered email and phone hashes in the gateway hashfmt (the "N/A" marker
// or standard base64 of exactly 32 bytes) plus the party-local resource
// references; plaintext PII never reaches the gateway. Register and Link
// are idempotent creates (one Idempotency-Key each); Resolve is a read and
// sends none. Errors surface raw and are rendered through
// gateway.MapError.
type IdentityClient interface {
	// Register registers one party-local subject's hashed identity at
	// POST /v1/identities/register through the SDK identities Register
	// route (party-signed, idempotent). A disabled adapter reports
	// ErrGatewayNotConfigured before any I/O; a non-hashfmt email or phone
	// hash fails closed before any request is sent.
	// Parameters: ctx bounds the call, resourceType the local resource type
	// (e.g. "patient"), localResourceID the party-local resource reference,
	// emailHash and phoneHash the peppered hashfmt values ("N/A" when
	// non-applicable).
	// Returns: the gateway's global_key grouping the equivalent
	// identities, plus the error to render through gateway.MapError. Never
	// panics.
	Register(ctx context.Context, resourceType, localResourceID, emailHash, phoneHash string) (globalKey string, err error)
	// Resolve resolves hashed identity fields to candidate links at
	// POST /v1/identities/resolve through the SDK identities Resolve route
	// — a read, so it sends no Idempotency-Key. A disabled adapter reports
	// ErrGatewayNotConfigured before any I/O; a non-hashfmt field fails
	// closed before any request is sent.
	// Parameters: ctx bounds the call, emailHash and phoneHash the
	// peppered hashfmt values ("N/A" when non-applicable).
	// Returns: the matching candidates (empty when the gateway reports
	// none), plus the error to render through gateway.MapError. Never
	// panics.
	Resolve(ctx context.Context, emailHash, phoneHash string) ([]IdentityCandidate, error)
	// Link declares equivalence between a confirmed global key and a
	// party-local resource at POST /v1/identities/link through the SDK
	// identities Link route (party-signed, idempotent). A disabled adapter
	// reports ErrGatewayNotConfigured before any I/O.
	// Parameters: ctx bounds the call, globalKey the confirmed candidate
	// key, localResourceID the party-local resource to link.
	// Returns: whether the gateway wrote the equivalence row, plus the
	// error to render through gateway.MapError. Never panics.
	Link(ctx context.Context, globalKey, localResourceID string) (linked bool, err error)
}

// Listing is one active marketplace listing row as the adapter surfaces it
// to contracts consumers — the backend-side mirror of the gateway read row;
// the SDK's own wire types never leave the gateway driver package.
type Listing struct {
	// ListingID is the listing's id (buyers reference it at payment create).
	ListingID string
	// ProviderPartyID is the providing party.
	ProviderPartyID string
	// Name is the listing title.
	Name string
	// Description is the marketplace detail ("" when unset).
	Description string
	// EndpointURL is set for commodity listings, empty for webhook rows.
	EndpointURL string
	// Price is the amount in IDR minor units; 0 marks a free listing.
	Price int64
	// Currency is the market currency ("IDR").
	Currency string
	// Invocation is "webhook" or "commodity".
	Invocation string
	// PriceScope is "party" or "user".
	PriceScope string
	// Metering is "gateway" or "provider".
	Metering string
	// ProviderTier is the operator trust tier ("" when unset).
	ProviderTier string
	// ProviderOrgName is the provider display name ("" when unset).
	ProviderOrgName string
}

// EnlistRequest is the listing to enlist at POST /v1/listings (BK-MARKET-001):
// a price change is a re-enlist through this request, never a redeploy.
type EnlistRequest struct {
	// Name is the listing title shown to buyers.
	Name string
	// Description is the optional marketplace detail.
	Description string
	// EndpointURL is where the buyer reaches the provider.
	EndpointURL string
	// Price is the amount in IDR minor units; 0 marks a free listing.
	Price int64
	// Invocation is "webhook" or "commodity" ("" lets the gateway default).
	Invocation string
	// PriceScope is "party" or "user" ("" lets the gateway default).
	PriceScope string
	// Metering is "gateway" or "provider" ("" lets the gateway default).
	Metering string
}

// ListingDirectory routes the marketplace listing endpoints (SDK-API-001,
// SDKP-007) through the adapter. Reads are memoized for listingCacheTTL so
// discovery and invocation lookups do not hammer the gateway. Errors surface
// raw (except a persistent miss, which fails closed with
// ErrListingNotFound) and are rendered through gateway.MapError.
type ListingDirectory interface {
	// FindListing resolves the single active listing named name provided by
	// provider through the SDK listings FindByName route: a cached row within
	// the TTL is served without I/O; on a miss exactly one refetch runs before
	// the lookup fails closed with ErrListingNotFound. A disabled adapter
	// reports ErrGatewayNotConfigured before any I/O.
	// Parameters: ctx bounds the lookup, provider the providing party, name
	// the listing title.
	// Returns: the active listing, or the error — ErrListingNotFound when the
	// gateway keeps reporting no match. Never panics.
	FindListing(ctx context.Context, provider, name string) (*Listing, error)
	// ListWebhookListings reads the active invocation=webhook listings for
	// discovery (GET /v1/listings filtered to webhook rows) and serves
	// repeated reads from its cache within the TTL. A disabled adapter reports
	// ErrGatewayNotConfigured before any I/O.
	// Parameters: ctx bounds the read.
	// Returns: the active webhook listings, plus the error to render through
	// MapError. Never panics.
	ListWebhookListings(ctx context.Context) ([]Listing, error)
	// Enlist creates or re-enlists one listing at POST /v1/listings through
	// the SDK listings Create route — party-signed and idempotent (one
	// Idempotency-Key per logical create). Any cached lookup for the enlisted
	// name is dropped so the next read sees the new listing. A disabled
	// adapter reports ErrGatewayNotConfigured before any I/O.
	// Parameters: ctx bounds the call, req the listing to enlist.
	// Returns: the created listing id, plus the error to render through
	// MapError. Never panics.
	Enlist(ctx context.Context, req EnlistRequest) (listingID string, err error)
}

// ApprovalClient routes the gateway approval plane (SDK-API-001,
// SDK-API-002): the outbound ticket mint and the inbound ticket check.
// Errors surface raw and are rendered through gateway.MapError.
type ApprovalClient interface {
	// Mint mints an approval handoff ticket at POST /v1/approvals through
	// the SDK approvals Mint route — party-signed and idempotent (one
	// Idempotency-Key per logical create, SDK-CLIENT-002). The body carries
	// the recipient party, the allowlisted scope, the literal purpose and
	// the researcher hash. A disabled adapter reports
	// ErrGatewayNotConfigured before any I/O.
	// Parameters: ctx bounds the call, to the recipient party id, scope the
	// allowlisted scope ("referral" or "export:<study>"), purpose the
	// literal purpose string, researcherHash the base64 32-byte researcher
	// hash or "N/A".
	// Returns: the minted ks-approval+jwt ticket (the response's
	// access_token), plus the error to render through gateway.MapError.
	// Never panics.
	Mint(ctx context.Context, to, scope, purpose, researcherHash string) (ticket string, err error)
	// VerifyTicket checks one directly presented ks-approval+jwt against
	// the gateway JWKS (APP_GATEWAY_URL/.well-known/jwks): exact typ
	// ks-approval+jwt, Ed25519 signature by the token's own header kid,
	// iss "gateway", aud equal to this party (APP_PARTY_ID), exp inside
	// the 60-second skew, scope exactly equal to expectedScope, and finally
	// a single-use jti consumed in the Redis replay cache — a replayed jti
	// or a Redis failure denies (fail closed). A disabled adapter reports
	// ErrGatewayNotConfigured before any I/O.
	// Parameters: ctx bounds the JWKS fetch and the jti reservation, token
	// the compact ticket JWS, expectedScope the exact scope of the
	// operation about to be performed.
	// Returns: nil once the ticket verifies for the expected scope, or the
	// error carrying the SDK sentinel (matchable with errors.Is). Never
	// panics.
	VerifyTicket(ctx context.Context, token, expectedScope string) error
}
