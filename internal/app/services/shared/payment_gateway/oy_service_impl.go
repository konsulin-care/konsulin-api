package payment_gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/dto/responses"
	"konsulin-service/internal/pkg/exceptions"
	"net/http"
)

type oyService struct {
	BaseUrl  string
	Username string
	ApiKey   string
}

func NewOyService(internalConfig *config.InternalConfig) (contracts.PaymentGatewayService, error) {
	return &oyService{
		BaseUrl:  internalConfig.PaymentGateway.BaseUrl,
		Username: internalConfig.PaymentGateway.Username,
		ApiKey:   internalConfig.PaymentGateway.ApiKey,
	}, nil
}

func (c *oyService) CreatePaymentRouting(ctx context.Context, request *requests.PaymentRequestDTO) (*responses.PaymentResponse, error) {
	requestJSON, err := json.Marshal(request)
	if err != nil {
		return nil, exceptions.ErrCannotMarshalJSON(err)
	}

	url := fmt.Sprintf("%s%s", c.BaseUrl, "payment-routing/create-transaction")

	req, err := http.NewRequestWithContext(ctx, constvars.MethodPost, url, bytes.NewBuffer(requestJSON))
	if err != nil {
		return nil, exceptions.ErrCreateHTTPRequest(err)
	}
	req.Header.Set(constvars.HeaderContentType, constvars.MIMEApplicationJSON)
	req.Header.Set(constvars.HeaderXOyUsername, c.Username)
	req.Header.Set(constvars.HeaderXApiKey, c.ApiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, exceptions.ErrSendHTTPRequest(err)
	}
	defer resp.Body.Close()

	paymentResponse := new(responses.PaymentResponse)
	err = json.NewDecoder(resp.Body).Decode(&paymentResponse)
	if err != nil {
		return nil, exceptions.ErrDecodeResponse(err, constvars.OyPaymentRoutingResource)
	}

	return paymentResponse, nil
}

func (c *oyService) CheckPaymentRoutingStatus(ctx context.Context, request *requests.PaymentRoutingStatus) (*responses.PaymentRoutingStatus, error) {
	requestJSON, err := json.Marshal(request)
	if err != nil {
		return nil, exceptions.ErrCannotMarshalJSON(err)
	}

	url := fmt.Sprintf("%s%s", c.BaseUrl, "payment-routing/check-status")

	req, err := http.NewRequestWithContext(ctx, constvars.MethodPost, url, bytes.NewBuffer(requestJSON))
	if err != nil {
		return nil, exceptions.ErrCreateHTTPRequest(err)
	}
	req.Header.Set(constvars.HeaderContentType, constvars.MIMEApplicationJSON)
	req.Header.Set(constvars.HeaderXOyUsername, c.Username)
	req.Header.Set(constvars.HeaderXApiKey, c.ApiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, exceptions.ErrSendHTTPRequest(err)
	}
	defer resp.Body.Close()

	paymentRoutingStatusResponse := new(responses.PaymentRoutingStatus)
	err = json.NewDecoder(resp.Body).Decode(&paymentRoutingStatusResponse)
	if err != nil {
		return nil, exceptions.ErrDecodeResponse(err, constvars.OyPaymentRoutingResource)
	}

	return paymentRoutingStatusResponse, nil
}
