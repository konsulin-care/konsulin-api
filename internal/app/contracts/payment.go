package contracts

import (
	"context"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/dto/responses"
)

type PaymentUsecase interface {
	PaymentRoutingCallback(ctx context.Context, request *requests.PaymentRoutingCallback) error
	CreatePay(ctx context.Context, request *requests.CreatePayRequest) (*responses.CreatePayResponse, error)
}
