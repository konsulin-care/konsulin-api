package contracts

import (
	"context"

	"github.com/konsulin-care/konsulin-api/internal/pkg/dto/requests"
	"github.com/konsulin-care/konsulin-api/internal/pkg/dto/responses"
)

// ServicePurchaseResult carries the created listing-mode payment: the
// gateway payment_id and its payment_url ("" when the gateway settles
// immediately and serves a null URL).
type ServicePurchaseResult struct {
	PaymentID  string `json:"payment_id"`
	PaymentURL string `json:"payment_url"`
}

type PaymentUsecase interface {
	// CreateServicePurchase resolves the requested marketplace listing
	// through the gateway listing directory (provider alias resolved to the
	// providing party id first) and creates the listing-mode payment for it,
	// carrying the caller's user_ref as customer_ref for price_scope=user
	// listings. An unknown provider or listing fails closed with a 404.
	// Parameters: ctx bounds the gateway calls, request names the provider
	// and the listing to buy.
	// Returns: the created payment_id and payment_url, or the error — a 404
	// for an unknown listing, a 503 when the gateway is not configured, the
	// gateway error otherwise. Never panics.
	CreateServicePurchase(ctx context.Context, request *requests.ServicePurchaseRequest) (*ServicePurchaseResult, error)
	HandleAppointmentPayment(ctx context.Context, request *requests.AppointmentPaymentRequest) (*responses.AppointmentPaymentResponse, error)
}
