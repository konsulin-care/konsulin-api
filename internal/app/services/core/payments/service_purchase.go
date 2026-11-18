package payments

import (
	"context"
	"errors"
	"strings"

	"github.com/konsulin-care/konsulin-api/internal/app/contracts"
	svc "github.com/konsulin-care/konsulin-api/internal/app/services/core/services"
	"github.com/konsulin-care/konsulin-api/internal/pkg/constvars"
	"github.com/konsulin-care/konsulin-api/internal/pkg/dto/requests"
	"github.com/konsulin-care/konsulin-api/internal/pkg/exceptions"
)

// servicePurchaseGatewayUnavailable renders the standard standalone-mode 503
// for a purchase attempted without a wired gateway client.
// Returns: the renderable 503 custom error. Never panics.
func servicePurchaseGatewayUnavailable() *exceptions.CustomError {
	return exceptions.BuildNewCustomError(
		contracts.ErrGatewayNotConfigured,
		constvars.StatusServiceUnavailable,
		"This endpoint requires the federation gateway. Configure APP_GATEWAY_URL and enroll this party to use it.",
		"GATEWAY_NOT_CONFIGURED",
	)
}

// servicePurchaseListingNotFound renders the 404 for an unknown provider or
// listing behind a purchase attempt.
// Returns: the renderable 404 custom error. Never panics.
func servicePurchaseListingNotFound(err error, devMessage string) *exceptions.CustomError {
	return exceptions.BuildNewCustomError(err, constvars.StatusNotFound, "Service not found", devMessage)
}

// servicePurchaseBodyInvalid renders the 400 for a missing provider or name.
// Returns: the renderable 400 custom error. Never panics.
func servicePurchaseBodyInvalid() *exceptions.CustomError {
	return exceptions.BuildNewCustomError(
		nil,
		constvars.StatusBadRequest,
		"provider and name are required",
		"SERVICE_PURCHASE_BODY_INVALID",
	)
}

// servicePurchaseValidate trims and checks the purchase payload.
// Parameters: req is the decoded purchase request (may be nil).
// Returns: the trimmed provider and name, or the 400 body error. Never panics.
func servicePurchaseValidate(req *requests.ServicePurchaseRequest) (string, string, error) {
	if req == nil {
		return "", "", servicePurchaseBodyInvalid()
	}
	provider := strings.TrimSpace(req.Provider)
	name := strings.TrimSpace(req.Name)
	if provider == "" || name == "" {
		return "", "", servicePurchaseBodyInvalid()
	}
	return provider, name, nil
}

// servicePurchaseResolveParty maps the provider alias to the providing party
// id through the resolver bound at bootstrap; a raw party id passes through
// when no resolver is wired.
// Parameters: provider is the alias or raw party id from the request.
// Returns: the party id to look the listing up under, or a 404 when the
// alias is unknown. Never panics.
func (uc *paymentUsecase) servicePurchaseResolveParty(provider string) (string, error) {
	if uc.Gateway.ResolveProvider == nil {
		return provider, nil
	}
	resolved, err := uc.Gateway.ResolveProvider(provider)
	if err != nil {
		return "", servicePurchaseListingNotFound(err, "PROVIDER_NOT_FOUND")
	}
	return resolved, nil
}

// servicePurchaseFindListing looks the listing up in the gateway listing
// directory and maps the persistent-miss and not-configured failures.
// Parameters: ctx bounds the gateway call, partyID identifies the providing
// party, name is the listing name to buy.
// Returns: the resolved listing, or a 404 for an unknown listing, a 503
// when the gateway clients are not wired, the gateway error otherwise.
// Never panics.
func (uc *paymentUsecase) servicePurchaseFindListing(
	ctx context.Context,
	partyID string,
	name string,
) (*contracts.Listing, error) {
	listing, err := uc.Gateway.Listings.FindListing(ctx, partyID, name)
	if errors.Is(err, contracts.ErrListingNotFound) {
		return nil, servicePurchaseListingNotFound(err, "LISTING_NOT_FOUND")
	}
	if errors.Is(err, contracts.ErrGatewayNotConfigured) {
		return nil, servicePurchaseGatewayUnavailable()
	}
	if err != nil {
		return nil, err
	}
	if listing == nil {
		return nil, servicePurchaseListingNotFound(nil, "LISTING_NOT_FOUND")
	}
	return listing, nil
}

// servicePurchaseCustomerRef derives the payment customer_ref for the
// resolved listing: empty for party-scoped listings, the caller's user_ref
// from the request context for price_scope=user listings (plaintext PII
// never reaches the gateway).
// Parameters: ctx carries the caller's user_ref, listing is the resolved
// listing to buy.
// Returns: the customer_ref to send, or a 400 when a user-scoped purchase
// has no caller user_ref. Never panics.
func servicePurchaseCustomerRef(ctx context.Context, listing *contracts.Listing) (string, error) {
	if !strings.EqualFold(strings.TrimSpace(listing.PriceScope), "user") {
		return "", nil
	}
	customerRef, _ := ctx.Value(svc.ContextKeyUserRef).(string)
	if strings.TrimSpace(customerRef) == "" {
		return "", exceptions.BuildNewCustomError(
			nil,
			constvars.StatusBadRequest,
			"user identity is required to purchase this service",
			"SERVICE_PURCHASE_USER_REF_MISSING",
		)
	}
	return customerRef, nil
}

// CreateServicePurchase resolves the requested marketplace listing through
// the gateway listing directory and creates the listing-mode payment for it.
// The provider alias is mapped to the providing party id through the
// resolver bound at bootstrap (a raw party id passes through); an unknown
// alias, an unknown listing or a persistent miss answers 404 before any
// payment call. For a price_scope=user listing the caller's user_ref from
// the request context is carried as customer_ref (plaintext PII never
// reaches the gateway); party-scoped listings send an empty customer_ref.
//
// Parameters:
//   - ctx carries the caller's user_ref and bounds the gateway calls.
//   - req names the provider alias (or raw party id) and the listing name.
//
// Returns: the created payment_id and payment_url, or the error — a 404 for
// an unknown provider/listing, a 503 when the gateway clients are not
// wired, a 400 when a user-scoped purchase has no caller user_ref, the
// gateway error otherwise. Never panics.
func (uc *paymentUsecase) CreateServicePurchase(
	ctx context.Context,
	req *requests.ServicePurchaseRequest,
) (*contracts.ServicePurchaseResult, error) {
	provider, name, err := servicePurchaseValidate(req)
	if err != nil {
		return nil, err
	}
	if uc.Gateway.Listings == nil || uc.Gateway.Payments == nil {
		return nil, servicePurchaseGatewayUnavailable()
	}
	partyID, err := uc.servicePurchaseResolveParty(provider)
	if err != nil {
		return nil, err
	}
	listing, err := uc.servicePurchaseFindListing(ctx, partyID, name)
	if err != nil {
		return nil, err
	}
	customerRef, err := servicePurchaseCustomerRef(ctx, listing)
	if err != nil {
		return nil, err
	}
	paymentID, paymentURL, payErr := uc.Gateway.Payments.CreateListingPayment(ctx, listing.ListingID, customerRef)
	if payErr != nil {
		return nil, payErr
	}
	return &contracts.ServicePurchaseResult{PaymentID: paymentID, PaymentURL: paymentURL}, nil
}
