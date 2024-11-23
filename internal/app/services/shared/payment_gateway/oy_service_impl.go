package payment_gateway

import (
	"context"
	"encoding/json"
	"konsulin-service/internal/app/config"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/dto/responses"
	"konsulin-service/internal/pkg/exceptions"
)

type oyService struct {
	BaseUrl  string
	Username string
	ApiKey   string
}

func NewOyService(internalConfig *config.InternalConfig) (PaymentGatewayService, error) {
	return &oyService{
		BaseUrl:  internalConfig.PaymentGateway.BaseUrl,
		Username: internalConfig.PaymentGateway.Username,
		ApiKey:   internalConfig.PaymentGateway.ApiKey,
	}, nil
}
func (s *oyService) CreatePaymentRouting(ctx context.Context, request *requests.PaymentRequest) (*responses.PaymentResponse, error) {
	_, err := json.Marshal(request)
	if err != nil {
		return nil, exceptions.ErrCannotMarshalJSON(err)
	}

	return nil, nil
}
