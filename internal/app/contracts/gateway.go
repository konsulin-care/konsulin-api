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
