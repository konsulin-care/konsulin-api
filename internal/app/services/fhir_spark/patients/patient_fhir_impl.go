package patients

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
	"net/url"
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
	client  *fhir_http_client.FHIRHTTPClient
}

func NewPatientFhirClient(baseUrl string, logger *zap.Logger) contracts.PatientFhirClient {
	oncePatientFhirClient.Do(func() {
		client := &patientFhirClient{
			BaseUrl: baseUrl + constvars.ResourcePatient,
			Log:     logger,
			client:  fhir_http_client.New(logger),
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

	respBody, err := c.client.Do(ctx, constvars.MethodPost, c.BaseUrl, bytes.NewBuffer(requestJSON))
	if err != nil {
		c.Log.Error("patientFhirClient.CreatePatient FHIR error",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCreateFHIRResource(err, constvars.ResourcePatient)
	}

	var patient fhir_dto.Patient
	if err := json.Unmarshal(respBody, &patient); err != nil {
		c.Log.Error("patientFhirClient.CreatePatient error decoding response",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourcePatient)
	}

	c.Log.Info("patientFhirClient.CreatePatient succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingPatientIDKey, patient.ID),
	)
	return &patient, nil
}

func (c *patientFhirClient) FindPatientByID(ctx context.Context, patientID string) (*fhir_dto.Patient, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	c.Log.Info("patientFhirClient.FindPatientByID called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingPatientIDKey, patientID),
	)

	respBody, err := c.client.Do(ctx, constvars.MethodGet, fmt.Sprintf("%s/%s", c.BaseUrl, patientID), nil)
	if err != nil {
		c.Log.Error("patientFhirClient.FindPatientByID FHIR error",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourcePatient)
	}

	var patient fhir_dto.Patient
	if err := json.Unmarshal(respBody, &patient); err != nil {
		c.Log.Error("patientFhirClient.FindPatientByID error decoding response",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourcePatient)
	}

	c.Log.Info("patientFhirClient.FindPatientByID succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingPatientIDKey, patient.ID),
	)
	return &patient, nil
}

func (c *patientFhirClient) FindPatientByIdentifier(ctx context.Context, identifier string) ([]fhir_dto.Patient, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	c.Log.Info("patientFhirClient.FindPatientByIdentifier called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	respBody, err := c.client.Do(ctx, constvars.MethodGet,
		fmt.Sprintf("%s?identifier=%s", c.BaseUrl, url.QueryEscape(identifier)), nil)
	if err != nil {
		c.Log.Error("patientFhirClient.FindPatientByIdentifier FHIR error",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourcePatient)
	}

	var result struct {
		Total        int    `json:"total"`
		ResourceType string `json:"resourceType"`
		Entry        []struct {
			FullUrl  string           `json:"fullUrl"`
			Resource fhir_dto.Patient `json:"resource"`
		} `json:"entry"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
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

	respBody, err := c.client.Do(ctx, constvars.MethodPut, fmt.Sprintf("%s/%s", c.BaseUrl, request.ID), bytes.NewBuffer(requestJSON))
	if err != nil {
		c.Log.Error("patientFhirClient.UpdatePatient FHIR error",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrUpdateFHIRResource(err, constvars.ResourcePatient)
	}

	var patient fhir_dto.Patient
	if err := json.Unmarshal(respBody, &patient); err != nil {
		c.Log.Error("patientFhirClient.UpdatePatient error decoding response",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourcePatient)
	}

	c.Log.Info("patientFhirClient.UpdatePatient succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingPatientIDKey, patient.ID),
	)
	return &patient, nil
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

	respBody, err := c.client.Do(ctx, constvars.MethodPatch, fmt.Sprintf("%s/%s", c.BaseUrl, request.ID), bytes.NewBuffer(requestJSON))
	if err != nil {
		c.Log.Error("patientFhirClient.PatchPatient FHIR error",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrUpdateFHIRResource(err, constvars.ResourcePatient)
	}

	var patient fhir_dto.Patient
	if err := json.Unmarshal(respBody, &patient); err != nil {
		c.Log.Error("patientFhirClient.PatchPatient error decoding response",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourcePatient)
	}

	c.Log.Info("patientFhirClient.PatchPatient succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingPatientIDKey, patient.ID),
	)
	return &patient, nil
}

// FindPatientByEmail queries Patient by email search parameter.
func (c *patientFhirClient) FindPatientByEmail(ctx context.Context, email string) ([]fhir_dto.Patient, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	c.Log.Info("patientFhirClient.FindPatientByEmail called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	emailEnc := url.QueryEscape(email)
	respBody, err := c.client.Do(ctx, constvars.MethodGet,
		fmt.Sprintf("%s?email=%s&_sort=-_lastUpdated", c.BaseUrl, emailEnc), nil)
	if err != nil {
		c.Log.Error("patientFhirClient.FindPatientByEmail FHIR error",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourcePatient)
	}

	var result struct {
		Total        int    `json:"total"`
		ResourceType string `json:"resourceType"`
		Entry        []struct {
			FullUrl  string           `json:"fullUrl"`
			Resource fhir_dto.Patient `json:"resource"`
		} `json:"entry"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		c.Log.Error("patientFhirClient.FindPatientByEmail error decoding response",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourcePatient)
	}

	patients := make([]fhir_dto.Patient, len(result.Entry))
	for i, entry := range result.Entry {
		patients[i] = entry.Resource
	}

	c.Log.Info("patientFhirClient.FindPatientByEmail succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Int(constvars.LoggingPatientCountKey, len(patients)),
	)
	return patients, nil
}

// FindPatientByPhone queries Patient by phone search parameter.
func (c *patientFhirClient) FindPatientByPhone(ctx context.Context, phone string) ([]fhir_dto.Patient, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	c.Log.Info("patientFhirClient.FindPatientByPhone called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	phoneEnc := url.QueryEscape(phone)
	respBody, err := c.client.Do(ctx, constvars.MethodGet,
		fmt.Sprintf("%s?phone=%s&_sort=-_lastUpdated", c.BaseUrl, phoneEnc), nil)
	if err != nil {
		c.Log.Error("patientFhirClient.FindPatientByPhone FHIR error",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourcePatient)
	}

	var result struct {
		Total        int    `json:"total"`
		ResourceType string `json:"resourceType"`
		Entry        []struct {
			FullUrl  string           `json:"fullUrl"`
			Resource fhir_dto.Patient `json:"resource"`
		} `json:"entry"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		c.Log.Error("patientFhirClient.FindPatientByPhone error decoding response",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourcePatient)
	}

	patients := make([]fhir_dto.Patient, len(result.Entry))
	for i, entry := range result.Entry {
		patients[i] = entry.Resource
	}

	c.Log.Info("patientFhirClient.FindPatientByPhone succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Int(constvars.LoggingPatientCountKey, len(patients)),
	)
	return patients, nil
}
