package gateway

import (
	"context"
	"fmt"

	"github.com/konsulin-care/konsulin-api/internal/app/contracts"

	"github.com/konsulin-care/gateway-sdk-go/payments"
	"github.com/konsulin-care/gateway-sdk-go/wire"
)

// paymentClientGuard proves at compile time that the adapter satisfies
// contracts.PaymentClient — the interface and the implementation must land
// together so the package compiles at every commit.
var _ contracts.PaymentClient = (*Client)(nil)

// paymentRoutes binds the SDK payment routes (SDK-API-001) to the adapter's
// SDK client. It fails closed: a disabled adapter reports
// contracts.ErrGatewayNotConfigured before any I/O.
// Parameters: none — reads the adapter's own state.
// Returns: the SDK payment routes, or the error explaining why they cannot
// be built. Never panics.
func (c *Client) paymentRoutes() (*payments.Client, error) {
	if !c.enabled {
		return nil, contracts.ErrGatewayNotConfigured
	}
	return payments.New(c.sdk)
}

// paymentURL unwraps the nullable payment_url of a create response: the
// free path settles immediately with an explicit JSON null, which the
// adapter surfaces as an empty string.
// Parameters: u is the pointer decoded from the create response.
// Returns: the provider checkout link, or "" for a nil pointer. Never
// panics.
func paymentURL(u *string) string {
	if u == nil {
		return ""
	}
	return *u
}

// CreateSellerPayment initiates a seller-amount payment at POST /v1/payments
// (seller-amount mode: domain + opaque + amount) through the SDK payments
// routes, party-signed by the adapter. The SDK marks the call idempotent,
// so one logical create carries one Idempotency-Key held stable across its
// retries (SDK-CLIENT-002). The body carries only the seller-mode fields —
// never customer_email or customer_name; the end user reaches the gateway
// only as a peppered customer_ref. A disabled adapter reports
// contracts.ErrGatewayNotConfigured before any I/O. Implements
// contracts.PaymentClient.
// Parameters: ctx bounds the call, domain the seller-mode domain, opaque
// the seller-mode opaque identifier, amount the amount in IDR minor units,
// customerRef the peppered end-user hash, successURL and cancelURL the
// buyer redirect targets.
// Returns: the created payment_id and its payment_url ("" when the gateway
// serves a null URL), plus the error — render it through MapError. Never
// panics.
func (c *Client) CreateSellerPayment(ctx context.Context, domain, opaque string, amount int64,
	customerRef, successURL, cancelURL string,
) (string, string, error) {
	routes, err := c.paymentRoutes()
	if err != nil {
		return "", "", fmt.Errorf("gateway: create seller payment: %w", err)
	}
	out, err := routes.Create(ctx, wire.CreatePaymentRequest{
		Domain:           domain,
		Opaque:           opaque,
		Amount:           amount,
		CustomerRef:      customerRef,
		SuccessReturnURL: successURL,
		CancelReturnURL:  cancelURL,
	})
	if err != nil {
		return "", "", fmt.Errorf("gateway: create seller payment: %w", err)
	}
	return out.PaymentID, paymentURL(out.PaymentURL), nil
}

// CreateListingPayment initiates a listing-mode payment at POST /v1/payments
// (listing mode: the gateway reads the price from the listing row) through
// the SDK payments routes, party-signed by the adapter. The SDK marks the
// call idempotent, so one logical create carries one Idempotency-Key held
// stable across its retries (SDK-CLIENT-002). The body carries listing_id
// and customer_ref only — never customer_email or customer_name. A disabled
// adapter reports contracts.ErrGatewayNotConfigured before any I/O.
// Implements contracts.PaymentClient.
// Parameters: ctx bounds the call, listingID the active listing to
// purchase, customerRef the peppered end-user hash.
// Returns: the created payment_id and its payment_url ("" when the gateway
// serves a null URL), plus the error — render it through MapError. Never
// panics.
func (c *Client) CreateListingPayment(ctx context.Context, listingID, customerRef string) (string, string, error) {
	routes, err := c.paymentRoutes()
	if err != nil {
		return "", "", fmt.Errorf("gateway: create listing payment: %w", err)
	}
	out, err := routes.Create(ctx, wire.CreatePaymentRequest{
		ListingID:   listingID,
		CustomerRef: customerRef,
	})
	if err != nil {
		return "", "", fmt.Errorf("gateway: create listing payment: %w", err)
	}
	return out.PaymentID, paymentURL(out.PaymentURL), nil
}

// GetPayment fetches one payment at GET /v1/payments/{id} through the SDK
// payments routes, party-signed by the adapter. As a read it sends no
// Idempotency-Key. A disabled adapter reports
// contracts.ErrGatewayNotConfigured before any I/O. Implements
// contracts.PaymentClient.
// Parameters: ctx bounds the call, paymentID the payment to reconcile.
// Returns: the gateway status ("PENDING", "PAID", "EXPIRED" or "CANCELED"),
// plus the error — render it through MapError. Never panics.
func (c *Client) GetPayment(ctx context.Context, paymentID string) (string, error) {
	routes, err := c.paymentRoutes()
	if err != nil {
		return "", fmt.Errorf("gateway: get payment: %w", err)
	}
	out, err := routes.Get(ctx, paymentID)
	if err != nil {
		return "", fmt.Errorf("gateway: get payment: %w", err)
	}
	return out.Status, nil
}
