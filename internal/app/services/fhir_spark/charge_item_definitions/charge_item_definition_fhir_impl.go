package charge_item_definitions

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/fhir_dto"
	"net/http"
	"sync"

	"go.uber.org/zap"
)

var (
	chargeItemDefinitionFhirClientInstance contracts.ChargeItemDefinitionFhirClient
	onceChargeItemDefinitionFhirClient     sync.Once
)

type chargeItemDefinitionFhirClient struct {
	BaseUrl string
	Log     *zap.Logger
}

func NewChargeItemDefinitionFhirClient(baseUrl string, logger *zap.Logger) contracts.ChargeItemDefinitionFhirClient {
	onceChargeItemDefinitionFhirClient.Do(func() {
		client := &chargeItemDefinitionFhirClient{
			BaseUrl: baseUrl + constvars.ResourceChargeItemDefinition,
			Log:     logger,
		}
		chargeItemDefinitionFhirClientInstance = client
	})
	return chargeItemDefinitionFhirClientInstance

}

func (c *chargeItemDefinitionFhirClient) CreateChargeItemDefinition(ctx context.Context, request *fhir_dto.ChargeItemDefinition) (*fhir_dto.ChargeItemDefinition, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	c.Log.Info("chargeItemDefinitionFhirClient.CreateChargeItemDefinition called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	requestJSON, err := json.Marshal(request)
	if err != nil {
		c.Log.Error("chargeItemDefinitionFhirClient.CreateChargeItemDefinition error marshaling JSON",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCannotMarshalJSON(err)
	}

	req, err := http.NewRequestWithContext(ctx, constvars.MethodPost, c.BaseUrl, bytes.NewBuffer(requestJSON))
	if err != nil {
		c.Log.Error("chargeItemDefinitionFhirClient.CreateChargeItemDefinition error creating HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCreateHTTPRequest(err)
	}
	req.Header.Set(constvars.HeaderContentType, constvars.MIMEApplicationFHIRJSON)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.Log.Error("chargeItemDefinitionFhirClient.CreateChargeItemDefinition error sending HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrSendHTTPRequest(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != constvars.StatusCreated {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			c.Log.Error("chargeItemDefinitionFhirClient.CreateChargeItemDefinition error reading response body",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, exceptions.ErrCreateFHIRResource(err, constvars.ResourceChargeItemDefinition)
		}

		var outcome fhir_dto.OperationOutcome
		err = json.Unmarshal(bodyBytes, &outcome)
		if err != nil {
			c.Log.Error("chargeItemDefinitionFhirClient.CreateChargeItemDefinition error unmarshaling outcome",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, exceptions.ErrCreateFHIRResource(err, constvars.ResourceChargeItemDefinition)
		}

		if len(outcome.Issue) > 0 {
			fhirErrorIssue := fmt.Errorf(outcome.Issue[0].Diagnostics)
			c.Log.Error("chargeItemDefinitionFhirClient.CreateChargeItemDefinition FHIR error",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(fhirErrorIssue),
			)
			return nil, exceptions.ErrCreateFHIRResource(fhirErrorIssue, constvars.ResourceChargeItemDefinition)
		}
	}

	chargeItemDefinitionFhir := new(fhir_dto.ChargeItemDefinition)
	err = json.NewDecoder(resp.Body).Decode(&chargeItemDefinitionFhir)
	if err != nil {
		c.Log.Error("chargeItemDefinitionFhirClient.CreateChargeItemDefinition error decoding response",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourceChargeItemDefinition)
	}

	c.Log.Info("chargeItemDefinitionFhirClient.CreateChargeItemDefinition succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingChargeItemDefinitionIDKey, chargeItemDefinitionFhir.ID),
	)
	return chargeItemDefinitionFhir, nil
}

func (c *chargeItemDefinitionFhirClient) FindChargeItemDefinitionByID(ctx context.Context, chargeItemDefinitionID string) (*fhir_dto.ChargeItemDefinition, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	c.Log.Info("chargeItemDefinitionFhirClient.FindChargeItemDefinitionByID called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingChargeItemDefinitionIDKey, chargeItemDefinitionID),
	)

	req, err := http.NewRequestWithContext(ctx, constvars.MethodGet, fmt.Sprintf("%s/%s", c.BaseUrl, chargeItemDefinitionID), nil)
	if err != nil {
		c.Log.Error("chargeItemDefinitionFhirClient.FindChargeItemDefinitionByID error creating HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCreateHTTPRequest(err)
	}
	req.Header.Set(constvars.HeaderContentType, constvars.MIMEApplicationFHIRJSON)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.Log.Error("chargeItemDefinitionFhirClient.FindChargeItemDefinitionByID error sending HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrSendHTTPRequest(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != constvars.StatusOK {
		if resp.StatusCode == constvars.StatusNotFound {
			c.Log.Info("chargeItemDefinitionFhirClient.FindChargeItemDefinitionByID not found",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.String(constvars.LoggingChargeItemDefinitionIDKey, chargeItemDefinitionID),
			)
			return &fhir_dto.ChargeItemDefinition{}, nil
		}
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			c.Log.Error("chargeItemDefinitionFhirClient.FindChargeItemDefinitionByID error reading response body",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourceChargeItemDefinition)
		}

		var outcome fhir_dto.OperationOutcome
		err = json.Unmarshal(bodyBytes, &outcome)
		if err != nil {
			c.Log.Error("chargeItemDefinitionFhirClient.FindChargeItemDefinitionByID error unmarshaling outcome",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourceChargeItemDefinition)
		}

		if len(outcome.Issue) > 0 {
			fhirErrorIssue := fmt.Errorf(outcome.Issue[0].Diagnostics)
			c.Log.Error("chargeItemDefinitionFhirClient.FindChargeItemDefinitionByID FHIR error",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(fhirErrorIssue),
			)
			return nil, exceptions.ErrGetFHIRResource(fhirErrorIssue, constvars.ResourceChargeItemDefinition)
		}
	}

	chargeItemDefinitionFhir := new(fhir_dto.ChargeItemDefinition)
	err = json.NewDecoder(resp.Body).Decode(&chargeItemDefinitionFhir)
	if err != nil {
		c.Log.Error("chargeItemDefinitionFhirClient.FindChargeItemDefinitionByID error decoding response",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourceChargeItemDefinition)
	}

	c.Log.Info("chargeItemDefinitionFhirClient.FindChargeItemDefinitionByID succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingChargeItemDefinitionIDKey, chargeItemDefinitionFhir.ID),
	)
	return chargeItemDefinitionFhir, nil
}

func (c *chargeItemDefinitionFhirClient) FindChargeItemDefinitionByPractitionerRoleID(ctx context.Context, chargeItemDefinitionID string) (*fhir_dto.ChargeItemDefinition, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	c.Log.Info("chargeItemDefinitionFhirClient.FindChargeItemDefinitionByPractitionerRoleID called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingChargeItemDefinitionIDKey, chargeItemDefinitionID),
	)

	req, err := http.NewRequestWithContext(ctx, constvars.MethodGet, fmt.Sprintf("%s/%s", c.BaseUrl, chargeItemDefinitionID), nil)
	if err != nil {
		c.Log.Error("chargeItemDefinitionFhirClient.FindChargeItemDefinitionByPractitionerRoleID error creating HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCreateHTTPRequest(err)
	}
	req.Header.Set(constvars.HeaderContentType, constvars.MIMEApplicationFHIRJSON)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.Log.Error("chargeItemDefinitionFhirClient.FindChargeItemDefinitionByPractitionerRoleID error sending HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrSendHTTPRequest(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != constvars.StatusOK {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			c.Log.Error("chargeItemDefinitionFhirClient.FindChargeItemDefinitionByPractitionerRoleID error reading response body",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourceChargeItemDefinition)
		}

		var outcome fhir_dto.OperationOutcome
		err = json.Unmarshal(bodyBytes, &outcome)
		if err != nil {
			c.Log.Error("chargeItemDefinitionFhirClient.FindChargeItemDefinitionByPractitionerRoleID error unmarshaling outcome",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourceChargeItemDefinition)
		}

		if len(outcome.Issue) > 0 {
			fhirErrorIssue := fmt.Errorf(outcome.Issue[0].Diagnostics)
			c.Log.Error("chargeItemDefinitionFhirClient.FindChargeItemDefinitionByPractitionerRoleID FHIR error",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(fhirErrorIssue),
			)
			return nil, exceptions.ErrGetFHIRResource(fhirErrorIssue, constvars.ResourceChargeItemDefinition)
		}
	}

	chargeItemDefinitionFhir := new(fhir_dto.ChargeItemDefinition)
	err = json.NewDecoder(resp.Body).Decode(&chargeItemDefinitionFhir)
	if err != nil {
		c.Log.Error("chargeItemDefinitionFhirClient.FindChargeItemDefinitionByPractitionerRoleID error decoding response",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourceChargeItemDefinition)
	}

	c.Log.Info("chargeItemDefinitionFhirClient.FindChargeItemDefinitionByPractitionerRoleID succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingChargeItemDefinitionIDKey, chargeItemDefinitionFhir.ID),
	)
	return chargeItemDefinitionFhir, nil
}

func (c *chargeItemDefinitionFhirClient) UpdateChargeItemDefinition(ctx context.Context, request *fhir_dto.ChargeItemDefinition) (*fhir_dto.ChargeItemDefinition, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	c.Log.Info("chargeItemDefinitionFhirClient.UpdateChargeItemDefinition called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	requestJSON, err := json.Marshal(request)
	if err != nil {
		c.Log.Error("chargeItemDefinitionFhirClient.UpdateChargeItemDefinition error marshaling JSON",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCannotMarshalJSON(err)
	}

	req, err := http.NewRequestWithContext(ctx, constvars.MethodPut, fmt.Sprintf("%s/%s", c.BaseUrl, request.ID), bytes.NewBuffer(requestJSON))
	if err != nil {
		c.Log.Error("chargeItemDefinitionFhirClient.UpdateChargeItemDefinition error creating HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCreateHTTPRequest(err)
	}
	req.Header.Set(constvars.HeaderContentType, constvars.MIMEApplicationFHIRJSON)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.Log.Error("chargeItemDefinitionFhirClient.UpdateChargeItemDefinition error sending HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrSendHTTPRequest(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			c.Log.Error("chargeItemDefinitionFhirClient.UpdateChargeItemDefinition error reading response body",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, exceptions.ErrUpdateFHIRResource(err, constvars.ResourceChargeItemDefinition)
		}

		var outcome fhir_dto.OperationOutcome
		err = json.Unmarshal(bodyBytes, &outcome)
		if err != nil {
			c.Log.Error("chargeItemDefinitionFhirClient.UpdateChargeItemDefinition error unmarshaling outcome",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, exceptions.ErrUpdateFHIRResource(err, constvars.ResourceChargeItemDefinition)
		}

		if len(outcome.Issue) > 0 {
			fhirErrorIssue := fmt.Errorf(outcome.Issue[0].Diagnostics)
			c.Log.Error("chargeItemDefinitionFhirClient.UpdateChargeItemDefinition FHIR error",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(fhirErrorIssue),
			)
			return nil, exceptions.ErrUpdateFHIRResource(fhirErrorIssue, constvars.ResourceChargeItemDefinition)
		}
	}

	chargeItemDefinitionFhir := new(fhir_dto.ChargeItemDefinition)
	err = json.NewDecoder(resp.Body).Decode(&chargeItemDefinitionFhir)
	if err != nil {
		c.Log.Error("chargeItemDefinitionFhirClient.UpdateChargeItemDefinition error decoding response",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourceChargeItemDefinition)
	}

	c.Log.Info("chargeItemDefinitionFhirClient.UpdateChargeItemDefinition succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingChargeItemDefinitionIDKey, chargeItemDefinitionFhir.ID),
	)
	return chargeItemDefinitionFhir, nil
}

func (c *chargeItemDefinitionFhirClient) PatchChargeItemDefinition(ctx context.Context, request *fhir_dto.ChargeItemDefinition) (*fhir_dto.ChargeItemDefinition, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	c.Log.Info("chargeItemDefinitionFhirClient.PatchChargeItemDefinition called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	requestJSON, err := json.Marshal(request)
	if err != nil {
		c.Log.Error("chargeItemDefinitionFhirClient.PatchChargeItemDefinition error marshaling JSON",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCannotMarshalJSON(err)
	}

	req, err := http.NewRequestWithContext(ctx, constvars.MethodPatch, fmt.Sprintf("%s/%s", c.BaseUrl, request.ID), bytes.NewBuffer(requestJSON))
	if err != nil {
		c.Log.Error("chargeItemDefinitionFhirClient.PatchChargeItemDefinition error creating HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCreateHTTPRequest(err)
	}
	req.Header.Set(constvars.HeaderContentType, constvars.MIMEApplicationFHIRJSON)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.Log.Error("chargeItemDefinitionFhirClient.PatchChargeItemDefinition error sending HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrSendHTTPRequest(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			c.Log.Error("chargeItemDefinitionFhirClient.PatchChargeItemDefinition error reading response body",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, exceptions.ErrUpdateFHIRResource(err, constvars.ResourceChargeItemDefinition)
		}

		var outcome fhir_dto.OperationOutcome
		err = json.Unmarshal(bodyBytes, &outcome)
		if err != nil {
			c.Log.Error("chargeItemDefinitionFhirClient.PatchChargeItemDefinition error unmarshaling outcome",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, exceptions.ErrUpdateFHIRResource(err, constvars.ResourceChargeItemDefinition)
		}

		if len(outcome.Issue) > 0 {
			fhirErrorIssue := fmt.Errorf(outcome.Issue[0].Diagnostics)
			c.Log.Error("chargeItemDefinitionFhirClient.PatchChargeItemDefinition FHIR error",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(fhirErrorIssue),
			)
			return nil, exceptions.ErrUpdateFHIRResource(fhirErrorIssue, constvars.ResourceChargeItemDefinition)
		}
	}

	chargeItemDefinitionFhir := new(fhir_dto.ChargeItemDefinition)
	err = json.NewDecoder(resp.Body).Decode(&chargeItemDefinitionFhir)
	if err != nil {
		c.Log.Error("chargeItemDefinitionFhirClient.PatchChargeItemDefinition error decoding response",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourceChargeItemDefinition)
	}

	c.Log.Info("chargeItemDefinitionFhirClient.PatchChargeItemDefinition succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingChargeItemDefinitionIDKey, chargeItemDefinitionFhir.ID),
	)
	return chargeItemDefinitionFhir, nil
}
