package contracts

import (
	"context"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/dto/responses"
)

type PaymentGatewayService interface {
	CreatePaymentRouting(ctx context.Context, request *requests.PaymentRequestDTO) (*responses.PaymentResponse, error)
	CheckPaymentRoutingStatus(ctx context.Context, request *requests.PaymentRoutingStatus) (*responses.PaymentRoutingStatus, error)
}
