package contracts

import (
	"context"
	"konsulin-service/internal/pkg/dto/requests"
)

type PaymentUsecase interface {
	PaymentRoutingCallback(ctx context.Context, request *requests.PaymentRoutingCallback) error
}
