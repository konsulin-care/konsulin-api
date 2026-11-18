package payments

import (
	"context"
	"testing"

	"github.com/konsulin-care/konsulin-api/internal/app/contracts"
	svc "github.com/konsulin-care/konsulin-api/internal/app/services/core/services"
	"github.com/konsulin-care/konsulin-api/internal/pkg/constvars"
	"github.com/konsulin-care/konsulin-api/internal/pkg/dto/requests"
	"github.com/konsulin-care/konsulin-api/internal/pkg/exceptions"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// purchaseListingCall records one FindListing invocation.
type purchaseListingCall struct {
	provider string
	name     string
}

// purchasePaymentCall records one CreateListingPayment invocation.
type purchasePaymentCall struct {
	listingID   string
	customerRef string
}

// fakeListingDirectory is a contracts.ListingDirectory double that records
// FindListing and answers with the configured listing or error.
type fakeListingDirectory struct {
	calls   []purchaseListingCall
	listing *contracts.Listing
	err     error
}

var _ contracts.ListingDirectory = (*fakeListingDirectory)(nil)

// FindListing records the lookup and answers with the configured listing or
// error. Never panics.
func (f *fakeListingDirectory) FindListing(_ context.Context, provider, name string) (*contracts.Listing, error) {
	f.calls = append(f.calls, purchaseListingCall{provider: provider, name: name})
	return f.listing, f.err
}

// ListWebhookListings fails the test run: purchases never read discovery.
func (*fakeListingDirectory) ListWebhookListings(context.Context) ([]contracts.Listing, error) {
	return nil, contracts.ErrGatewayNotConfigured
}

// Enlist fails the test run: purchases never enlist listings.
func (*fakeListingDirectory) Enlist(context.Context, contracts.EnlistRequest) (string, error) {
	return "", contracts.ErrGatewayNotConfigured
}

// fakePurchasePaymentClient is a contracts.PaymentClient double that records
// listing-mode creates and answers with the configured payment values.
type fakePurchasePaymentClient struct {
	calls      []purchasePaymentCall
	paymentID  string
	paymentURL string
	err        error
}

var _ contracts.PaymentClient = (*fakePurchasePaymentClient)(nil)

// CreateListingPayment records the listing-mode create and answers with the
// configured payment id and payment URL. Never panics.
func (f *fakePurchasePaymentClient) CreateListingPayment(_ context.Context, listingID, customerRef string) (string, string, error) {
	f.calls = append(f.calls, purchasePaymentCall{listingID: listingID, customerRef: customerRef})
	return f.paymentID, f.paymentURL, f.err
}

// CreateSellerPayment fails the test run: purchases use listing mode only.
func (*fakePurchasePaymentClient) CreateSellerPayment(context.Context, string, string, int64, string, string, string) (string, string, error) {
	return "", "", exceptions.ErrServerProcess(nil)
}

// GetPayment fails the test run: purchases never reconcile a payment.
func (*fakePurchasePaymentClient) GetPayment(context.Context, string) (string, error) {
	return "", exceptions.ErrServerProcess(nil)
}

// newPurchaseUsecase builds a paymentUsecase carrying only the gateway deps
// the purchase flow reads: the listing directory, the payment client and the
// provider alias resolver.
func newPurchaseUsecase(directory contracts.ListingDirectory, pay contracts.PaymentClient,
	resolve func(string) (string, error),
) *paymentUsecase {
	return &paymentUsecase{Gateway: GatewayDeps{
		Listings:        directory,
		Payments:        pay,
		ResolveProvider: resolve,
	}}
}

// TestCreateServicePurchaseCreatesListingPaymentAndReturnsIDs covers
// assertion 1: a service purchase resolves the listing through the gateway
// deps (provider alias resolved to the providing party id) and creates a
// listing-mode payment for it, returning payment_id and payment_url.
func TestCreateServicePurchaseCreatesListingPaymentAndReturnsIDs(t *testing.T) {
	directory := &fakeListingDirectory{listing: &contracts.Listing{
		ListingID:  "lst-1",
		PriceScope: "party",
		Invocation: "webhook",
	}}
	pay := &fakePurchasePaymentClient{
		paymentID:  "pay-abc-1",
		paymentURL: "https://pay.gateway.test/checkout/pay-abc-1",
	}
	uc := newPurchaseUsecase(directory, pay, func(alias string) (string, error) {
		if alias == "automation" {
			return "pty-automation", nil
		}
		return alias, nil
	})

	result, err := uc.CreateServicePurchase(context.Background(), &requests.ServicePurchaseRequest{
		Provider: "automation",
		Name:     "interpret",
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "pay-abc-1", result.PaymentID, "must return the gateway payment_id")
	assert.Equal(t, "https://pay.gateway.test/checkout/pay-abc-1", result.PaymentURL, "must return the gateway payment_url")

	require.Len(t, directory.calls, 1, "the listing must be resolved exactly once")
	assert.Equal(t, "pty-automation", directory.calls[0].provider, "the provider alias must resolve to the party id")
	assert.Equal(t, "interpret", directory.calls[0].name, "the listing name must be looked up")

	require.Len(t, pay.calls, 1, "one listing-mode payment must be created")
	assert.Equal(t, "lst-1", pay.calls[0].listingID, "the payment must target the resolved listing")
	assert.Empty(t, pay.calls[0].customerRef, "a party-scoped listing sends no customer_ref")
}

// TestCreateServicePurchaseUserScopedCarriesCallerUserRef covers
// assertion 2: a user-scoped listing purchase carries the caller's user_ref
// from the request context as the payment customer_ref.
func TestCreateServicePurchaseUserScopedCarriesCallerUserRef(t *testing.T) {
	directory := &fakeListingDirectory{listing: &contracts.Listing{
		ListingID:  "lst-user-1",
		PriceScope: "user",
	}}
	pay := &fakePurchasePaymentClient{paymentID: "pay-user-1", paymentURL: "https://pay.gateway.test/checkout/pay-user-1"}
	uc := newPurchaseUsecase(directory, pay, func(alias string) (string, error) { return alias, nil })

	ctx := context.WithValue(context.Background(), svc.ContextKeyUserRef, "peppered-user-ref")
	result, err := uc.CreateServicePurchase(ctx, &requests.ServicePurchaseRequest{
		Provider: "pty-provider",
		Name:     "interpret",
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, pay.calls, 1)
	assert.Equal(t, "peppered-user-ref", pay.calls[0].customerRef,
		"a user-scoped listing must carry the caller's user_ref as customer_ref")
}

// TestCreateServicePurchaseUnknownListingAnswers404 covers assertion 3: an
// unknown listing answers 404 and no payment is created.
func TestCreateServicePurchaseUnknownListingAnswers404(t *testing.T) {
	directory := &fakeListingDirectory{err: contracts.ErrListingNotFound}
	pay := &fakePurchasePaymentClient{paymentID: "pay-never", paymentURL: "https://pay.gateway.test/never"}
	uc := newPurchaseUsecase(directory, pay, func(alias string) (string, error) { return alias, nil })

	_, err := uc.CreateServicePurchase(context.Background(), &requests.ServicePurchaseRequest{
		Provider: "pty-provider",
		Name:     "no-such-listing",
	})

	require.Error(t, err)
	var customErr *exceptions.CustomError
	require.ErrorAs(t, err, &customErr)
	assert.Equal(t, constvars.StatusNotFound, customErr.StatusCode, "an unknown listing must answer 404")
	assert.Empty(t, pay.calls, "no payment may be created for an unknown listing")
}
