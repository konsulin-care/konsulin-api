package practitioners

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
	practitionerFhirClientInstance contracts.PractitionerFhirClient
	oncePractitionerFhirClient     sync.Once
)

type practitionerFhirClient struct {
	BaseUrl string
	Log     *zap.Logger
}

func NewPractitionerFhirClient(baseUrl string, logger *zap.Logger) contracts.PractitionerFhirClient {
	oncePractitionerFhirClient.Do(func() {
		client := &practitionerFhirClient{
			BaseUrl: baseUrl + constvars.ResourcePractitioner,
			Log:     logger,
		}
		practitionerFhirClientInstance = client
	})
	return practitionerFhirClientInstance
}

func (c *practitionerFhirClient) CreatePractitioner(ctx context.Context, request *fhir_dto.Practitioner) (*fhir_dto.Practitioner, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	c.Log.Info("practitionerFhirClient.CreatePractitioner called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	requestJSON, err := json.Marshal(request)
	if err != nil {
		c.Log.Error("practitionerFhirClient.CreatePractitioner error marshaling JSON",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCannotMarshalJSON(err)
	}

	req, err := http.NewRequestWithContext(ctx, constvars.MethodPost, c.BaseUrl, bytes.NewBuffer(requestJSON))
	if err != nil {
		c.Log.Error("practitionerFhirClient.CreatePractitioner error creating HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCreateHTTPRequest(err)
	}
	req.Header.Set(constvars.HeaderContentType, constvars.MIMEApplicationFHIRJSON)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.Log.Error("practitionerFhirClient.CreatePractitioner error sending HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrSendHTTPRequest(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != constvars.StatusCreated {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			c.Log.Error("practitionerFhirClient.CreatePractitioner error reading response body",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, exceptions.ErrCreateFHIRResource(err, constvars.ResourcePractitioner)
		}
		var outcome fhir_dto.OperationOutcome
		err = json.Unmarshal(bodyBytes, &outcome)
		if err != nil {
			c.Log.Error("practitionerFhirClient.CreatePractitioner error unmarshaling outcome",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, exceptions.ErrCreateFHIRResource(err, constvars.ResourcePractitioner)
		}
		if len(outcome.Issue) > 0 {
			fhirErrorIssue := fmt.Errorf(outcome.Issue[0].Diagnostics)
			c.Log.Error("practitionerFhirClient.CreatePractitioner FHIR error",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(fhirErrorIssue),
			)
			return nil, exceptions.ErrCreateFHIRResource(fhirErrorIssue, constvars.ResourcePractitioner)
		}
	}

	practitionerFhir := new(fhir_dto.Practitioner)
	err = json.NewDecoder(resp.Body).Decode(&practitionerFhir)
	if err != nil {
		c.Log.Error("practitionerFhirClient.CreatePractitioner error decoding response",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourcePractitioner)
	}

	c.Log.Info("practitionerFhirClient.CreatePractitioner succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingPractitionerIDKey, practitionerFhir.ID),
	)
	return practitionerFhir, nil
}

func (c *practitionerFhirClient) FindPractitionerByID(ctx context.Context, practitionerID string) (*fhir_dto.Practitioner, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	c.Log.Info("practitionerFhirClient.FindPractitionerByID called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingPractitionerIDKey, practitionerID),
	)

	req, err := http.NewRequestWithContext(ctx, constvars.MethodGet,
		fmt.Sprintf("%s/%s", c.BaseUrl, practitionerID), nil)
	if err != nil {
		c.Log.Error("practitionerFhirClient.FindPractitionerByID error creating HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCreateHTTPRequest(err)
	}
	req.Header.Set(constvars.HeaderContentType, constvars.MIMEApplicationFHIRJSON)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.Log.Error("practitionerFhirClient.FindPractitionerByID error sending HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrSendHTTPRequest(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != constvars.StatusOK {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			c.Log.Error("practitionerFhirClient.FindPractitionerByID error reading response body",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourcePractitioner)
		}
		var outcome fhir_dto.OperationOutcome
		err = json.Unmarshal(bodyBytes, &outcome)
		if err != nil {
			c.Log.Error("practitionerFhirClient.FindPractitionerByID error unmarshaling outcome",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourcePractitioner)
		}
		if len(outcome.Issue) > 0 {
			fhirErrorIssue := fmt.Errorf(outcome.Issue[0].Diagnostics)
			c.Log.Error("practitionerFhirClient.FindPractitionerByID FHIR error",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(fhirErrorIssue),
			)
			return nil, exceptions.ErrGetFHIRResource(fhirErrorIssue, constvars.ResourcePractitioner)
		}
	}

	practitionerFhir := new(fhir_dto.Practitioner)
	err = json.NewDecoder(resp.Body).Decode(&practitionerFhir)
	if err != nil {
		c.Log.Error("practitionerFhirClient.FindPractitionerByID error decoding response",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourcePractitioner)
	}

	c.Log.Info("practitionerFhirClient.FindPractitionerByID succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingPractitionerIDKey, practitionerFhir.ID),
	)
	return practitionerFhir, nil
}

func (c *practitionerFhirClient) UpdatePractitioner(ctx context.Context, request *fhir_dto.Practitioner) (*fhir_dto.Practitioner, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	c.Log.Info("practitionerFhirClient.UpdatePractitioner called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	requestJSON, err := json.Marshal(request)
	if err != nil {
		c.Log.Error("practitionerFhirClient.UpdatePractitioner error marshaling JSON",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCannotMarshalJSON(err)
	}

	req, err := http.NewRequestWithContext(ctx, constvars.MethodPut,
		fmt.Sprintf("%s/%s", c.BaseUrl, request.ID),
		bytes.NewBuffer(requestJSON),
	)
	if err != nil {
		c.Log.Error("practitionerFhirClient.UpdatePractitioner error creating HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCreateHTTPRequest(err)
	}
	req.Header.Set(constvars.HeaderContentType, constvars.MIMEApplicationFHIRJSON)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.Log.Error("practitionerFhirClient.UpdatePractitioner error sending HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrSendHTTPRequest(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != constvars.StatusOK {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			c.Log.Error("practitionerFhirClient.UpdatePractitioner error reading response body",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, exceptions.ErrUpdateFHIRResource(err, constvars.ResourcePractitioner)
		}
		var outcome fhir_dto.OperationOutcome
		err = json.Unmarshal(bodyBytes, &outcome)
		if err != nil {
			c.Log.Error("practitionerFhirClient.UpdatePractitioner error unmarshaling outcome",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, exceptions.ErrUpdateFHIRResource(err, constvars.ResourcePractitioner)
		}
		if len(outcome.Issue) > 0 {
			fhirErrorIssue := fmt.Errorf(outcome.Issue[0].Diagnostics)
			c.Log.Error("practitionerFhirClient.UpdatePractitioner FHIR error",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(fhirErrorIssue),
			)
			return nil, exceptions.ErrUpdateFHIRResource(fhirErrorIssue, constvars.ResourcePractitioner)
		}
	}

	practitionerFhir := new(fhir_dto.Practitioner)
	err = json.NewDecoder(resp.Body).Decode(&practitionerFhir)
	if err != nil {
		c.Log.Error("practitionerFhirClient.UpdatePractitioner error decoding response",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourcePractitioner)
	}

	c.Log.Info("practitionerFhirClient.UpdatePractitioner succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingPractitionerIDKey, practitionerFhir.ID),
	)
	return practitionerFhir, nil
}

func (c *practitionerFhirClient) PatchPractitioner(ctx context.Context, request *fhir_dto.Practitioner) (*fhir_dto.Practitioner, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	c.Log.Info("practitionerFhirClient.PatchPractitioner called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	requestJSON, err := json.Marshal(request)
	if err != nil {
		c.Log.Error("practitionerFhirClient.PatchPractitioner error marshaling JSON",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCannotMarshalJSON(err)
	}

	req, err := http.NewRequestWithContext(ctx, constvars.MethodPatch,
		fmt.Sprintf("%s/%s", c.BaseUrl, request.ID),
		bytes.NewBuffer(requestJSON),
	)
	if err != nil {
		c.Log.Error("practitionerFhirClient.PatchPractitioner error creating HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCreateHTTPRequest(err)
	}
	req.Header.Set(constvars.HeaderContentType, constvars.MIMEApplicationFHIRJSON)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.Log.Error("practitionerFhirClient.PatchPractitioner error sending HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrSendHTTPRequest(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != constvars.StatusOK {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			c.Log.Error("practitionerFhirClient.PatchPractitioner error reading response body",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, exceptions.ErrUpdateFHIRResource(err, constvars.ResourcePractitioner)
		}
		var outcome fhir_dto.OperationOutcome
		err = json.Unmarshal(bodyBytes, &outcome)
		if err != nil {
			c.Log.Error("practitionerFhirClient.PatchPractitioner error unmarshaling outcome",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, exceptions.ErrUpdateFHIRResource(err, constvars.ResourcePractitioner)
		}
		if len(outcome.Issue) > 0 {
			fhirErrorIssue := fmt.Errorf(outcome.Issue[0].Diagnostics)
			c.Log.Error("practitionerFhirClient.PatchPractitioner FHIR error",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(fhirErrorIssue),
			)
			return nil, exceptions.ErrUpdateFHIRResource(fhirErrorIssue, constvars.ResourcePractitioner)
		}
	}

	practitionerFhir := new(fhir_dto.Practitioner)
	err = json.NewDecoder(resp.Body).Decode(&practitionerFhir)
	if err != nil {
		c.Log.Error("practitionerFhirClient.PatchPractitioner error decoding response",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourcePractitioner)
	}

	c.Log.Info("practitionerFhirClient.PatchPractitioner succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingPractitionerIDKey, practitionerFhir.ID),
	)
	return practitionerFhir, nil
}
