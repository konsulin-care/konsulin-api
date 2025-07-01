package patients

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
	patientFhirClientInstance contracts.PatientFhirClient
	oncePatientFhirClient     sync.Once
)

type patientFhirClient struct {
	BaseUrl string
	Log     *zap.Logger
}

func NewPatientFhirClient(baseUrl string, logger *zap.Logger) contracts.PatientFhirClient {
	oncePatientFhirClient.Do(func() {
		client := &patientFhirClient{
			BaseUrl: baseUrl + constvars.ResourcePatient,
			Log:     logger,
		}
		patientFhirClientInstance = client
	})
	return patientFhirClientInstance
}

func (c *patientFhirClient) CreatePatient(ctx context.Context, request *fhir_dto.Patient) (*fhir_dto.Patient, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	c.Log.Info("patientFhirClient.CreatePatient called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	requestJSON, err := json.Marshal(request)
	if err != nil {
		c.Log.Error("patientFhirClient.CreatePatient error marshaling JSON",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCannotMarshalJSON(err)
	}

	req, err := http.NewRequestWithContext(ctx, constvars.MethodPost, c.BaseUrl, bytes.NewBuffer(requestJSON))
	if err != nil {
		c.Log.Error("patientFhirClient.CreatePatient error creating HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCreateHTTPRequest(err)
	}
	req.Header.Set(constvars.HeaderContentType, constvars.MIMEApplicationFHIRJSON)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.Log.Error("patientFhirClient.CreatePatient error sending HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrSendHTTPRequest(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != constvars.StatusCreated {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			c.Log.Error("patientFhirClient.CreatePatient error reading response body",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, exceptions.ErrCreateFHIRResource(err, constvars.ResourcePatient)
		}

		var outcome fhir_dto.OperationOutcome
		err = json.Unmarshal(bodyBytes, &outcome)
		if err != nil {
			c.Log.Error("patientFhirClient.CreatePatient error unmarshaling outcome",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, exceptions.ErrCreateFHIRResource(err, constvars.ResourcePatient)
		}

		if len(outcome.Issue) > 0 {
			fhirErrorIssue := fmt.Errorf(outcome.Issue[0].Diagnostics)
			c.Log.Error("patientFhirClient.CreatePatient FHIR error",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(fhirErrorIssue),
			)
			return nil, exceptions.ErrCreateFHIRResource(fhirErrorIssue, constvars.ResourcePatient)
		}
	}

	patientFhir := new(fhir_dto.Patient)
	err = json.NewDecoder(resp.Body).Decode(&patientFhir)
	if err != nil {
		c.Log.Error("patientFhirClient.CreatePatient error decoding response",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourcePatient)
	}

	c.Log.Info("patientFhirClient.CreatePatient succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingPatientIDKey, patientFhir.ID),
	)
	return patientFhir, nil
}

func (c *patientFhirClient) FindPatientByID(ctx context.Context, patientID string) (*fhir_dto.Patient, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	c.Log.Info("patientFhirClient.FindPatientByID called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingPatientIDKey, patientID),
	)

	req, err := http.NewRequestWithContext(ctx, constvars.MethodGet, fmt.Sprintf("%s/%s", c.BaseUrl, patientID), nil)
	if err != nil {
		c.Log.Error("patientFhirClient.FindPatientByID error creating HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCreateHTTPRequest(err)
	}
	req.Header.Set(constvars.HeaderContentType, constvars.MIMEApplicationFHIRJSON)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.Log.Error("patientFhirClient.FindPatientByID error sending HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrSendHTTPRequest(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != constvars.StatusOK {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			c.Log.Error("patientFhirClient.FindPatientByID error reading response body",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourcePatient)
		}

		var outcome fhir_dto.OperationOutcome
		err = json.Unmarshal(bodyBytes, &outcome)
		if err != nil {
			c.Log.Error("patientFhirClient.FindPatientByID error unmarshaling outcome",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourcePatient)
		}

		if len(outcome.Issue) > 0 {
			fhirErrorIssue := fmt.Errorf(outcome.Issue[0].Diagnostics)
			c.Log.Error("patientFhirClient.FindPatientByID FHIR error",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(fhirErrorIssue),
			)
			return nil, exceptions.ErrGetFHIRResource(fhirErrorIssue, constvars.ResourcePatient)
		}
	}

	patientFhir := new(fhir_dto.Patient)
	err = json.NewDecoder(resp.Body).Decode(&patientFhir)
	if err != nil {
		c.Log.Error("patientFhirClient.FindPatientByID error decoding response",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourcePatient)
	}

	c.Log.Info("patientFhirClient.FindPatientByID succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingPatientIDKey, patientFhir.ID),
	)
	return patientFhir, nil
}

func (c *patientFhirClient) FindPatientByIdentifier(ctx context.Context, system, value string) ([]fhir_dto.Patient, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	c.Log.Info("patientFhirClient.FindPatientByIdentifier called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	req, err := http.NewRequestWithContext(ctx, constvars.MethodGet,
		fmt.Sprintf("%s?identifier=%s|%s", c.BaseUrl, system, value), nil)
	if err != nil {
		c.Log.Error("patientFhirClient.FindPatientByIdentifier error creating HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCreateHTTPRequest(err)
	}
	req.Header.Set(constvars.HeaderContentType, constvars.MIMEApplicationFHIRJSON)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.Log.Error("patientFhirClient.FindPatientByIdentifier error sending HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrSendHTTPRequest(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != constvars.StatusOK {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			c.Log.Error("patientFhirClient.FindPatientByIdentifier error reading response body",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourcePatient)
		}
		var outcome fhir_dto.OperationOutcome
		err = json.Unmarshal(bodyBytes, &outcome)
		if err != nil {
			c.Log.Error("patientFhirClient.FindPatientByIdentifier error unmarshaling outcome",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourcePatient)
		}
		if len(outcome.Issue) > 0 {
			fhirErrorIssue := fmt.Errorf(outcome.Issue[0].Diagnostics)
			c.Log.Error("patientFhirClient.FindPatientByIdentifier FHIR error",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(fhirErrorIssue),
			)
			return nil, exceptions.ErrGetFHIRResource(fhirErrorIssue, constvars.ResourcePatient)
		}
	}

	var result struct {
		Total        int    `json:"total"`
		ResourceType string `json:"resourceType"`
		Entry        []struct {
			FullUrl  string           `json:"fullUrl"`
			Resource fhir_dto.Patient `json:"resource"`
		} `json:"entry"`
	}
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		c.Log.Error("patientFhirClient.FindPatientByIdentifier error decoding response",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourcePatient)
	}

	patients := make([]fhir_dto.Patient, len(result.Entry))
	for i, entry := range result.Entry {
		patients[i] = entry.Resource
	}

	c.Log.Info("patientFhirClient.FindPatientByIdentifier succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Int(constvars.LoggingPatientCountKey, len(patients)),
	)
	return patients, nil
}

func (c *patientFhirClient) UpdatePatient(ctx context.Context, request *fhir_dto.Patient) (*fhir_dto.Patient, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	c.Log.Info("patientFhirClient.UpdatePatient called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	requestJSON, err := json.Marshal(request)
	if err != nil {
		c.Log.Error("patientFhirClient.UpdatePatient error marshaling JSON",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCannotMarshalJSON(err)
	}

	req, err := http.NewRequestWithContext(ctx, constvars.MethodPut, fmt.Sprintf("%s/%s", c.BaseUrl, request.ID), bytes.NewBuffer(requestJSON))
	if err != nil {
		c.Log.Error("patientFhirClient.UpdatePatient error creating HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCreateHTTPRequest(err)
	}
	req.Header.Set(constvars.HeaderContentType, constvars.MIMEApplicationFHIRJSON)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.Log.Error("patientFhirClient.UpdatePatient error sending HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrSendHTTPRequest(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			c.Log.Error("patientFhirClient.UpdatePatient error reading response body",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, exceptions.ErrUpdateFHIRResource(err, constvars.ResourcePatient)
		}

		var outcome fhir_dto.OperationOutcome
		err = json.Unmarshal(bodyBytes, &outcome)
		if err != nil {
			c.Log.Error("patientFhirClient.UpdatePatient error unmarshaling outcome",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, exceptions.ErrUpdateFHIRResource(err, constvars.ResourcePatient)
		}

		if len(outcome.Issue) > 0 {
			fhirErrorIssue := fmt.Errorf(outcome.Issue[0].Diagnostics)
			c.Log.Error("patientFhirClient.UpdatePatient FHIR error",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(fhirErrorIssue),
			)
			return nil, exceptions.ErrUpdateFHIRResource(fhirErrorIssue, constvars.ResourcePatient)
		}
	}

	patientFhir := new(fhir_dto.Patient)
	err = json.NewDecoder(resp.Body).Decode(&patientFhir)
	if err != nil {
		c.Log.Error("patientFhirClient.UpdatePatient error decoding response",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourcePatient)
	}

	c.Log.Info("patientFhirClient.UpdatePatient succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingPatientIDKey, patientFhir.ID),
	)
	return patientFhir, nil
}

func (c *patientFhirClient) PatchPatient(ctx context.Context, request *fhir_dto.Patient) (*fhir_dto.Patient, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	c.Log.Info("patientFhirClient.PatchPatient called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	requestJSON, err := json.Marshal(request)
	if err != nil {
		c.Log.Error("patientFhirClient.PatchPatient error marshaling JSON",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCannotMarshalJSON(err)
	}

	req, err := http.NewRequestWithContext(ctx, constvars.MethodPatch, fmt.Sprintf("%s/%s", c.BaseUrl, request.ID), bytes.NewBuffer(requestJSON))
	if err != nil {
		c.Log.Error("patientFhirClient.PatchPatient error creating HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCreateHTTPRequest(err)
	}
	req.Header.Set(constvars.HeaderContentType, constvars.MIMEApplicationFHIRJSON)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.Log.Error("patientFhirClient.PatchPatient error sending HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrSendHTTPRequest(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			c.Log.Error("patientFhirClient.PatchPatient error reading response body",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, exceptions.ErrUpdateFHIRResource(err, constvars.ResourcePatient)
		}

		var outcome fhir_dto.OperationOutcome
		err = json.Unmarshal(bodyBytes, &outcome)
		if err != nil {
			c.Log.Error("patientFhirClient.PatchPatient error unmarshaling outcome",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, exceptions.ErrUpdateFHIRResource(err, constvars.ResourcePatient)
		}

		if len(outcome.Issue) > 0 {
			fhirErrorIssue := fmt.Errorf(outcome.Issue[0].Diagnostics)
			c.Log.Error("patientFhirClient.PatchPatient FHIR error",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(fhirErrorIssue),
			)
			return nil, exceptions.ErrUpdateFHIRResource(fhirErrorIssue, constvars.ResourcePatient)
		}
	}

	patientFhir := new(fhir_dto.Patient)
	err = json.NewDecoder(resp.Body).Decode(&patientFhir)
	if err != nil {
		c.Log.Error("patientFhirClient.PatchPatient error decoding response",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourcePatient)
	}

	c.Log.Info("patientFhirClient.PatchPatient succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingPatientIDKey, patientFhir.ID),
	)
	return patientFhir, nil
}
