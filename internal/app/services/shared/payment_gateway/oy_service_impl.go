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
	"sync"

	"go.uber.org/zap"
)

var (
	oyServiceInstance contracts.PaymentGatewayService
	onceOyService     sync.Once
)

type oyService struct {
	BaseUrl  string
	Username string
	ApiKey   string
	Log      *zap.Logger
}

func NewOyService(internalConfig *config.InternalConfig, logger *zap.Logger) contracts.PaymentGatewayService {
	onceOyService.Do(func() {
		instance := &oyService{
			BaseUrl:  internalConfig.PaymentGateway.BaseUrl,
			Username: internalConfig.PaymentGateway.Username,
			ApiKey:   internalConfig.PaymentGateway.ApiKey,
			Log:      logger,
		}
		oyServiceInstance = instance
	})

	return oyServiceInstance
}
func (c *oyService) CreatePaymentRouting(ctx context.Context, request *requests.PaymentRequestDTO) (*responses.PaymentResponse, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	c.Log.Info("oyService.CreatePaymentRouting called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	requestJSON, err := json.Marshal(request)
	if err != nil {
		c.Log.Error("oyService.CreatePaymentRouting error marshaling JSON",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCannotMarshalJSON(err)
	}

	url := fmt.Sprintf("%s%s", c.BaseUrl, "/api/payment-routing/create-transaction")
	c.Log.Info("oyService.CreatePaymentRouting built URL",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String("url", url),
	)

	req, err := http.NewRequestWithContext(ctx, constvars.MethodPost, url, bytes.NewBuffer(requestJSON))
	if err != nil {
		c.Log.Error("oyService.CreatePaymentRouting error creating HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCreateHTTPRequest(err)
	}
	req.Header.Set(constvars.HeaderContentType, constvars.MIMEApplicationJSON)
	req.Header.Set(constvars.HeaderXOyUsername, c.Username)
	req.Header.Set(constvars.HeaderXApiKey, c.ApiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.Log.Error("oyService.CreatePaymentRouting error sending HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrSendHTTPRequest(err)
	}
	defer resp.Body.Close()

	paymentResponse := new(responses.PaymentResponse)
	err = json.NewDecoder(resp.Body).Decode(&paymentResponse)
	if err != nil {
		c.Log.Error("oyService.CreatePaymentRouting error decoding response",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrDecodeResponse(err, constvars.OyPaymentRoutingResource)
	}

	successStatusCode := "000"
	if paymentResponse.Status.Code != successStatusCode {
		c.Log.Error("oyService.CreatePaymentRouting received non success status code on create payment routing",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String("status", paymentResponse.Status.Code),
			zap.String("message", paymentResponse.Status.Message),
		)
		return nil, exceptions.ErrClientCustomMessage(fmt.Errorf("received non success status code: %s", paymentResponse.Status.Code))
	}

	c.Log.Info("oyService.CreatePaymentRouting succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingOyPaymentID, paymentResponse.TrxID),
	)
	return paymentResponse, nil
}

func (c *oyService) CheckPaymentRoutingStatus(ctx context.Context, request *requests.PaymentRoutingStatus) (*responses.PaymentRoutingStatus, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	c.Log.Info("oyService.CheckPaymentRoutingStatus called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	requestJSON, err := json.Marshal(request)
	if err != nil {
		c.Log.Error("oyService.CheckPaymentRoutingStatus error marshaling JSON",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCannotMarshalJSON(err)
	}

	url := fmt.Sprintf("%s%s", c.BaseUrl, "payment-routing/check-status")
	c.Log.Info("oyService.CheckPaymentRoutingStatus built URL",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingOyUrlKey, url),
	)

	req, err := http.NewRequestWithContext(ctx, constvars.MethodPost, url, bytes.NewBuffer(requestJSON))
	if err != nil {
		c.Log.Error("oyService.CheckPaymentRoutingStatus error creating HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCreateHTTPRequest(err)
	}
	req.Header.Set(constvars.HeaderContentType, constvars.MIMEApplicationJSON)
	req.Header.Set(constvars.HeaderXOyUsername, c.Username)
	req.Header.Set(constvars.HeaderXApiKey, c.ApiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.Log.Error("oyService.CheckPaymentRoutingStatus error sending HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrSendHTTPRequest(err)
	}
	defer resp.Body.Close()

	paymentRoutingStatusResponse := new(responses.PaymentRoutingStatus)
	err = json.NewDecoder(resp.Body).Decode(&paymentRoutingStatusResponse)
	if err != nil {
		c.Log.Error("oyService.CheckPaymentRoutingStatus error decoding response",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrDecodeResponse(err, constvars.OyPaymentRoutingResource)
	}

	c.Log.Info("oyService.CheckPaymentRoutingStatus succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingOyPaymentStatusKey, paymentRoutingStatusResponse.PaymentStatus),
	)
	return paymentRoutingStatusResponse, nil
}
