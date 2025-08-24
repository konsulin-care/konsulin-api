package service_requests

import (
	"bytes"
	"context"
	"encoding/json"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/fhir_dto"
	"net/http"
	"sync"

	"go.uber.org/zap"
)

var (
	serviceRequestFhirClientInstance contracts.ServiceRequestFhirClient
	onceServiceRequestFhirClient     sync.Once
)

type serviceRequestFhirClient struct {
	BaseUrl string
	Log     *zap.Logger
}

func NewServiceRequestFhirClient(baseUrl string, logger *zap.Logger) contracts.ServiceRequestFhirClient {
	onceServiceRequestFhirClient.Do(func() {
		client := &serviceRequestFhirClient{
			BaseUrl: baseUrl + constvars.ResourceServiceRequest,
			Log:     logger,
		}
		serviceRequestFhirClientInstance = client
	})
	return serviceRequestFhirClientInstance
}

func (c *serviceRequestFhirClient) CreateServiceRequest(ctx context.Context, request *fhir_dto.CreateServiceRequestInput) (*fhir_dto.CreateServiceRequestOutput, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	c.Log.Info("serviceRequestFhirClient.CreateServiceRequest called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	requestJSON, err := json.Marshal(request)
	if err != nil {
		c.Log.Error("serviceRequestFhirClient.CreateServiceRequest error marshaling JSON",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCannotMarshalJSON(err)
	}

	req, err := http.NewRequestWithContext(ctx, constvars.MethodPost, c.BaseUrl, bytes.NewBuffer(requestJSON))
	if err != nil {
		c.Log.Error("serviceRequestFhirClient.CreateServiceRequest error creating HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCreateHTTPRequest(err)
	}
	req.Header.Set(constvars.HeaderContentType, constvars.MIMEApplicationFHIRJSON)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.Log.Error("serviceRequestFhirClient.CreateServiceRequest error sending HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrSendHTTPRequest(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != constvars.StatusCreated {
		return nil, exceptions.ErrCreateFHIRResource(nil, constvars.ResourceServiceRequest)
	}

	serviceRequest := new(fhir_dto.CreateServiceRequestOutput)
	if err := json.NewDecoder(resp.Body).Decode(&serviceRequest); err != nil {
		c.Log.Error("serviceRequestFhirClient.CreateServiceRequest error decoding response",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourceServiceRequest)
	}

	c.Log.Info("serviceRequestFhirClient.CreateServiceRequest succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String("service_request_id", serviceRequest.ID),
	)
	return serviceRequest, nil
}
