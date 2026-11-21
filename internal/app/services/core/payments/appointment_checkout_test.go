package payments

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/konsulin-care/konsulin-api/internal/app/contracts"
	"github.com/konsulin-care/konsulin-api/internal/pkg/constvars"
	"github.com/konsulin-care/konsulin-api/internal/pkg/exceptions"
	"github.com/konsulin-care/konsulin-api/internal/pkg/fhir_dto"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// checkoutIDs are the six canonical appointment ids the checkout tests use.
var checkoutIDs = []string{"slot-1", "pr-2", "pat-3", "inv-4", "hs-5", "appt-6"}

// checkoutFrontendDomain is the APP_FRONTEND_DOMAIN value the checkout sends
// as success and cancel return URL.
const checkoutFrontendDomain = "https://app.konsulin.test"

// checkoutSellerCall records one CreateSellerPayment invocation.
type checkoutSellerCall struct {
	domain      string
	opaque      string
	amount      int64
	customerRef string
	successURL  string
	cancelURL   string
}

// fakeCheckoutPaymentClient is a contracts.PaymentClient double that records
// every seller-mode create and answers with configured payment values.
type fakeCheckoutPaymentClient struct {
	calls      []checkoutSellerCall
	paymentID  string
	paymentURL string
	err        error
}

var _ contracts.PaymentClient = (*fakeCheckoutPaymentClient)(nil)

// CreateSellerPayment records the seller-mode create and answers with the
// configured payment id and payment URL.
func (f *fakeCheckoutPaymentClient) CreateSellerPayment(_ context.Context, domain, opaque string,
	amount int64, customerRef, successURL, cancelURL string,
) (string, string, error) {
	f.calls = append(f.calls, checkoutSellerCall{
		domain:      domain,
		opaque:      opaque,
		amount:      amount,
		customerRef: customerRef,
		successURL:  successURL,
		cancelURL:   cancelURL,
	})
	return f.paymentID, f.paymentURL, f.err
}

// CreateListingPayment fails the test run: checkout must use seller mode only.
func (*fakeCheckoutPaymentClient) CreateListingPayment(context.Context, string, string) (string, string, error) {
	return "", "", errors.New("unexpected CreateListingPayment call")
}

// GetPayment fails the test run: checkout never reconciles a payment.
func (*fakeCheckoutPaymentClient) GetPayment(context.Context, string) (string, error) {
	return "", errors.New("unexpected GetPayment call")
}

// newCheckoutInput builds the checkout input carrying the six canonical ids,
// the given invoice and the APP_FRONTEND_DOMAIN return-URL domain.
func newCheckoutInput(invoice *fhir_dto.Invoice) AppointmentCheckoutInput {
	return AppointmentCheckoutInput{
		SlotID:              checkoutIDs[0],
		PractitionerRoleID:  checkoutIDs[1],
		PatientID:           checkoutIDs[2],
		InvoiceID:           checkoutIDs[3],
		HealthcareServiceID: checkoutIDs[4],
		AppointmentID:       checkoutIDs[5],
		Invoice:             invoice,
		FrontendDomain:      checkoutFrontendDomain,
	}
}

// newCheckoutInvoice builds an IDR invoice with the given totalNet value.
func newCheckoutInvoice(totalNet float64) *fhir_dto.Invoice {
	return &fhir_dto.Invoice{
		ResourceType: constvars.ResourceInvoice,
		ID:           checkoutIDs[3],
		TotalNet:     &fhir_dto.Money{Value: totalNet, Currency: constvars.CurrencyIndonesianRupiah},
	}
}

// TestCreateAppointmentCheckoutCreatesSellerPaymentAndReturnsURL covers
// assertion 1: the checkout creates a seller-mode gateway payment with domain
// "appointment", the six-id opaque and amount ceil(Invoice.totalNet) whole
// rupiah, sends the APP_FRONTEND_DOMAIN return URLs without any customer
// email/name, and returns the gateway payment_url.
func TestCreateAppointmentCheckoutCreatesSellerPaymentAndReturnsURL(t *testing.T) {
	fake := &fakeCheckoutPaymentClient{
		paymentID:  "pay-abc-123",
		paymentURL: "https://pay.gateway.test/checkout/pay-abc-123",
	}
	invoice := newCheckoutInvoice(150000.4)
	var persisted *fhir_dto.Invoice
	persist := func(_ context.Context, inv *fhir_dto.Invoice) error {
		persisted = inv
		return nil
	}

	url, err := CreateAppointmentCheckout(context.Background(), fake, persist, newCheckoutInput(invoice))

	require.NoError(t, err)
	assert.Equal(t, "https://pay.gateway.test/checkout/pay-abc-123", url)

	require.Len(t, fake.calls, 1)
	call := fake.calls[0]
	assert.Equal(t, "appointment", call.domain)
	wantOpaque, opaqueErr := BuildAppointmentOpaque(checkoutIDs)
	require.NoError(t, opaqueErr)
	assert.Equal(t, wantOpaque, call.opaque)
	assert.Equal(t, int64(150001), call.amount, "amount must be ceil(totalNet) in whole rupiah")
	assert.Equal(t, checkoutFrontendDomain, call.successURL)
	assert.Equal(t, checkoutFrontendDomain, call.cancelURL)
	assert.Empty(t, call.customerRef, "no customer email/name may reach the gateway")
	require.NotNil(t, persisted, "the invoice must be handed to the persistence callback")
}

// TestCreateAppointmentCheckoutRejectsNonIDRCurrencyBeforeGatewayCall covers
// assertion 2: an invoice in a currency other than IDR is rejected with 400
// and the gateway is never called.
func TestCreateAppointmentCheckoutRejectsNonIDRCurrencyBeforeGatewayCall(t *testing.T) {
	fake := &fakeCheckoutPaymentClient{
		paymentID:  "pay-never",
		paymentURL: "https://pay.gateway.test/checkout/pay-never",
	}
	invoice := newCheckoutInvoice(150000)
	invoice.TotalNet.Currency = "USD"
	persistCalled := false
	persist := func(context.Context, *fhir_dto.Invoice) error {
		persistCalled = true
		return nil
	}

	_, err := CreateAppointmentCheckout(context.Background(), fake, persist, newCheckoutInput(invoice))

	require.Error(t, err)
	var customErr *exceptions.CustomError
	require.ErrorAs(t, err, &customErr)
	assert.Equal(t, 400, customErr.StatusCode)
	assert.Empty(t, fake.calls, "no gateway call may happen for a non-IDR invoice")
	assert.False(t, persistCalled, "nothing may be persisted for a rejected invoice")
}

// TestCreateAppointmentCheckoutMapsGatewayNotConfiguredTo503 covers assertion
// 4: a disabled gateway adapter (wrapped contracts.ErrGatewayNotConfigured,
// exactly as the SDK adapter reports it in standalone mode) is rendered as a
// 503 CustomError instead of a generic 500, and nothing is persisted.
func TestCreateAppointmentCheckoutMapsGatewayNotConfiguredTo503(t *testing.T) {
	fake := &fakeCheckoutPaymentClient{
		err: fmt.Errorf("gateway: create seller payment: %w", contracts.ErrGatewayNotConfigured),
	}
	persistCalled := false
	persist := func(context.Context, *fhir_dto.Invoice) error {
		persistCalled = true
		return nil
	}

	_, err := CreateAppointmentCheckout(context.Background(), fake, persist, newCheckoutInput(newCheckoutInvoice(150000)))

	require.Error(t, err)
	var customErr *exceptions.CustomError
	require.ErrorAs(t, err, &customErr)
	assert.Equal(t, constvars.StatusServiceUnavailable, customErr.StatusCode)
	assert.False(t, persistCalled, "nothing may be persisted when the gateway is not configured")
}

// TestCreateAppointmentCheckoutMapsGatewayTransportErrorTo502 covers assertion
// 5: a transport failure from the gateway client is rendered as a 502
// CustomError, and nothing is persisted.
func TestCreateAppointmentCheckoutMapsGatewayTransportErrorTo502(t *testing.T) {
	fake := &fakeCheckoutPaymentClient{
		err: errors.New("dial tcp: connection refused"),
	}
	persistCalled := false
	persist := func(context.Context, *fhir_dto.Invoice) error {
		persistCalled = true
		return nil
	}

	_, err := CreateAppointmentCheckout(context.Background(), fake, persist, newCheckoutInput(newCheckoutInvoice(150000)))

	require.Error(t, err)
	var customErr *exceptions.CustomError
	require.ErrorAs(t, err, &customErr)
	assert.Equal(t, constvars.StatusBadGateway, customErr.StatusCode)
	assert.False(t, persistCalled, "nothing may be persisted when the gateway call fails")
}

// TestCreateAppointmentCheckoutPersistsPaymentIDAsInvoiceIdentifier covers
// assertion 3: the gateway payment_id is persisted on the Invoice as an
// identifier with system urn:konsulin:gateway-payment.
func TestCreateAppointmentCheckoutPersistsPaymentIDAsInvoiceIdentifier(t *testing.T) {
	fake := &fakeCheckoutPaymentClient{
		paymentID:  "pay-persist-9",
		paymentURL: "https://pay.gateway.test/checkout/pay-persist-9",
	}
	invoice := newCheckoutInvoice(250000)
	var persisted *fhir_dto.Invoice
	persist := func(_ context.Context, inv *fhir_dto.Invoice) error {
		persisted = inv
		return nil
	}

	_, err := CreateAppointmentCheckout(context.Background(), fake, persist, newCheckoutInput(invoice))

	require.NoError(t, err)
	require.NotNil(t, persisted)
	var found *fhir_dto.Identifier
	for i := range persisted.Identifier {
		if persisted.Identifier[i].System == "urn:konsulin:gateway-payment" {
			found = &persisted.Identifier[i]
			break
		}
	}
	require.NotNil(t, found, "payment_id must be persisted with system urn:konsulin:gateway-payment")
	assert.Equal(t, "pay-persist-9", found.Value)
}
