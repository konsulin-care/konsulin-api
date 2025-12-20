package contracts

import (
	"context"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/dto/responses"
)

type PaymentUsecase interface {
	PaymentRoutingCallback(ctx context.Context, request *requests.PaymentRoutingCallback) error
	CreatePay(ctx context.Context, request *requests.CreatePayRequest) (*responses.CreatePayResponse, error)
	HandleAppointmentPayment(ctx context.Context, request *requests.AppointmentPaymentRequest) (*responses.AppointmentPaymentResponse, error)
	XenditInvoiceCallback(ctx context.Context, header *requests.XenditInvoiceCallbackHeader, body *requests.XenditInvoiceCallbackBody) error
}
