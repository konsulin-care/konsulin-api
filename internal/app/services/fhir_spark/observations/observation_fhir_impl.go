package observations

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/fhir_dto"
	"konsulin-service/internal/pkg/fhir_http_client"
	"sync"

	"go.uber.org/zap"
)

var (
	observationFhirClientInstance contracts.ObservationFhirClient
	onceObservationFhirClient     sync.Once
)

type observationFhirClient struct {
	BaseUrl string
	Log     *zap.Logger
	client  *fhir_http_client.FHIRHTTPClient
}

func NewObservationFhirClient(baseUrl string, logger *zap.Logger) contracts.ObservationFhirClient {
	onceObservationFhirClient.Do(func() {
		client := &observationFhirClient{
			BaseUrl: baseUrl + constvars.ResourceObservation,
			Log:     logger,
			client:  fhir_http_client.New(logger),
		}
		observationFhirClientInstance = client
	})
	return observationFhirClientInstance
}

func (c *observationFhirClient) CreateObservation(ctx context.Context, request *fhir_dto.Observation) (*fhir_dto.Observation, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	c.Log.Info("observationFhirClient.CreateObservation called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	requestJSON, err := json.Marshal(request)
	if err != nil {
		c.Log.Error("observationFhirClient.CreateObservation error marshaling JSON",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCannotMarshalJSON(err)
	}

	respBody, err := c.client.Do(ctx, constvars.MethodPost, c.BaseUrl, bytes.NewBuffer(requestJSON))
	if err != nil {
		c.Log.Error("observationFhirClient.CreateObservation FHIR error",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCreateFHIRResource(err, constvars.ResourceObservation)
	}

	var observation fhir_dto.Observation
	if err := json.Unmarshal(respBody, &observation); err != nil {
		c.Log.Error("observationFhirClient.CreateObservation error decoding response",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourceObservation)
	}

	c.Log.Info("observationFhirClient.CreateObservation succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingObservationIDKey, observation.ID),
	)
	return &observation, nil
}

func (c *observationFhirClient) FindObservationByID(ctx context.Context, observationID string) (*fhir_dto.Observation, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	c.Log.Info("observationFhirClient.FindObservationByID called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingObservationIDKey, observationID),
	)

	respBody, err := c.client.Do(ctx, constvars.MethodGet, fmt.Sprintf("%s/%s", c.BaseUrl, observationID), nil)
	if err != nil {
		c.Log.Error("observationFhirClient.FindObservationByID FHIR error",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourceObservation)
	}

	var observation fhir_dto.Observation
	if err := json.Unmarshal(respBody, &observation); err != nil {
		c.Log.Error("observationFhirClient.FindObservationByID error decoding response",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourceObservation)
	}

	c.Log.Info("observationFhirClient.FindObservationByID succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingObservationIDKey, observation.ID),
	)
	return &observation, nil
}

func (c *observationFhirClient) DeleteObservationByID(ctx context.Context, observationID string) error {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	c.Log.Info("observationFhirClient.DeleteObservationByID called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingObservationIDKey, observationID),
	)

	_, err := c.client.Do(ctx, constvars.MethodDelete, fmt.Sprintf("%s/%s", c.BaseUrl, observationID), nil)
	if err != nil {
		c.Log.Error("observationFhirClient.DeleteObservationByID FHIR error",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return exceptions.ErrGetFHIRResource(err, constvars.ResourceObservation)
	}

	c.Log.Info("observationFhirClient.DeleteObservationByID succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingObservationIDKey, observationID),
	)
	return nil
}

func (c *observationFhirClient) UpdateObservation(ctx context.Context, request *fhir_dto.Observation) (*fhir_dto.Observation, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	c.Log.Info("observationFhirClient.UpdateObservation called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	requestJSON, err := json.Marshal(request)
	if err != nil {
		c.Log.Error("observationFhirClient.UpdateObservation error marshaling JSON",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCannotMarshalJSON(err)
	}

	respBody, err := c.client.Do(ctx, constvars.MethodPut, fmt.Sprintf("%s/%s", c.BaseUrl, request.ID), bytes.NewBuffer(requestJSON))
	if err != nil {
		c.Log.Error("observationFhirClient.UpdateObservation FHIR error",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrUpdateFHIRResource(err, constvars.ResourceObservation)
	}

	var observation fhir_dto.Observation
	if err := json.Unmarshal(respBody, &observation); err != nil {
		c.Log.Error("observationFhirClient.UpdateObservation error decoding response",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourceObservation)
	}

	c.Log.Info("observationFhirClient.UpdateObservation succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingObservationIDKey, observation.ID),
	)
	return &observation, nil
}

func (c *observationFhirClient) PatchObservation(ctx context.Context, request *fhir_dto.Observation) (*fhir_dto.Observation, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	c.Log.Info("observationFhirClient.PatchObservation called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	requestJSON, err := json.Marshal(request)
	if err != nil {
		c.Log.Error("observationFhirClient.PatchObservation error marshaling JSON",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCannotMarshalJSON(err)
	}

	respBody, err := c.client.Do(ctx, constvars.MethodPatch, fmt.Sprintf("%s/%s", c.BaseUrl, request.ID), bytes.NewBuffer(requestJSON))
	if err != nil {
		c.Log.Error("observationFhirClient.PatchObservation FHIR error",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrUpdateFHIRResource(err, constvars.ResourceObservation)
	}

	var observation fhir_dto.Observation
	if err := json.Unmarshal(respBody, &observation); err != nil {
		c.Log.Error("observationFhirClient.PatchObservation error decoding response",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourceObservation)
	}

	c.Log.Info("observationFhirClient.PatchObservation succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingObservationIDKey, observation.ID),
	)
	return &observation, nil
}
