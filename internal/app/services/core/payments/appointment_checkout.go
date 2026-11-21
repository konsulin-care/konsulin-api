package payments

import (
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/konsulin-care/konsulin-api/internal/app/contracts"
	"github.com/konsulin-care/konsulin-api/internal/app/drivers/gateway"
	"github.com/konsulin-care/konsulin-api/internal/pkg/constvars"
	"github.com/konsulin-care/konsulin-api/internal/pkg/exceptions"
	"github.com/konsulin-care/konsulin-api/internal/pkg/fhir_dto"
)

// GatewayPaymentIdentifierSystem is the FHIR identifier system under which the
// gateway payment_id is recorded on the Invoice.
const GatewayPaymentIdentifierSystem = "urn:konsulin:gateway-payment"

// AppointmentCheckoutInput carries everything the standalone appointment
// checkout needs: the six appointment identifiers (slot, practitionerRole,
// patient, invoice, healthcareService, appointment), the invoice whose
// totalNet sets the amount, and the APP_FRONTEND_DOMAIN value used for the
// success and cancel return URLs.
type AppointmentCheckoutInput struct {
	SlotID              string
	PractitionerRoleID  string
	PatientID           string
	InvoiceID           string
	HealthcareServiceID string
	AppointmentID       string
	Invoice             *fhir_dto.Invoice
	FrontendDomain      string
}

// InvoicePersistenceFunc persists the invoice — carrying the gateway payment
// identifier — to durable storage. It is injected so this checkout stays a
// standalone function with no FHIR client dependency.
type InvoicePersistenceFunc func(ctx context.Context, invoice *fhir_dto.Invoice) error

// CreateAppointmentCheckout creates the seller-mode gateway payment for an
// appointment booking and persists the gateway payment_id on the invoice.
//
// The payment uses domain "appointment" and the opaque built by
// BuildAppointmentOpaque over the six appointment ids, with the amount set to
// ceil(Invoice.totalNet) in whole rupiah. Only IDR invoices are accepted: any
// other currency is rejected with a 400 before any gateway call. The create
// carries the APP_FRONTEND_DOMAIN success/cancel return URLs and no customer
// email or name. After a successful create the payment_id is appended to the
// invoice as an identifier with system urn:konsulin:gateway-payment and the
// invoice is handed to the persistence callback.
//
// Parameters:
//   - ctx bounds the gateway call and the persistence callback.
//   - client creates the seller-mode gateway payment.
//   - persist stores the invoice carrying the new payment identifier.
//   - input supplies the six appointment ids, the invoice and the return domain.
//
// Returns the gateway payment_url, or an error: a 400-style CustomError for a
// rejected invoice or malformed ids, the gateway error rendered through
// gateway.MapError (503 when the gateway is not configured or the party is not
// enrolled, the gateway's own status for a gateway-returned error, 502 for a
// transport failure), or the persistence error otherwise.
func CreateAppointmentCheckout(
	ctx context.Context,
	client contracts.PaymentClient,
	persist InvoicePersistenceFunc,
	input AppointmentCheckoutInput,
) (string, error) {
	if input.Invoice == nil || input.Invoice.TotalNet == nil {
		return "", exceptions.BuildNewCustomError(
			nil,
			constvars.StatusBadRequest,
			"Invoice total amount is required",
			"checkout invoice or totalNet missing",
		)
	}
	currency := strings.TrimSpace(input.Invoice.TotalNet.Currency)
	if !strings.EqualFold(currency, constvars.CurrencyIndonesianRupiah) {
		return "", exceptions.BuildNewCustomError(
			fmt.Errorf("currency %q is not supported", currency),
			constvars.StatusBadRequest,
			"Only IDR payments are supported",
			fmt.Sprintf("invoice currency %q rejected before gateway call", currency),
		)
	}

	opaque, err := BuildAppointmentOpaque([]string{
		input.SlotID,
		input.PractitionerRoleID,
		input.PatientID,
		input.InvoiceID,
		input.HealthcareServiceID,
		input.AppointmentID,
	})
	if err != nil {
		return "", exceptions.BuildNewCustomError(
			err,
			constvars.StatusBadRequest,
			"Invalid appointment identifiers",
			err.Error(),
		)
	}

	amount := int64(math.Ceil(input.Invoice.TotalNet.Value))

	// No customer email/name: the request body carries no customer PII.
	paymentID, paymentURL, gatewayErr := client.CreateSellerPayment(
		ctx,
		string(constvars.AppointmentPaymentService),
		opaque,
		amount,
		"",
		input.FrontendDomain,
		input.FrontendDomain,
	)
	if gatewayErr != nil {
		return "", gateway.MapError(gatewayErr)
	}

	input.Invoice.Identifier = append(
		input.Invoice.Identifier,
		fhir_dto.Identifier{System: GatewayPaymentIdentifierSystem, Value: paymentID},
	)
	if persistErr := persist(ctx, input.Invoice); persistErr != nil {
		return "", persistErr
	}

	return paymentURL, nil
}
