package patients

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"konsulin-service/internal/app/models"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/exceptions"
	"net/http"
)

type patientFhirClient struct {
	BaseUrl string
}

func NewPatientFhirClient(patientFhirBaseUrl string) PatientFhirClient {
	return &patientFhirClient{
		BaseUrl: patientFhirBaseUrl,
	}
}

func (c *patientFhirClient) CreatePatient(ctx context.Context, request *requests.PatientFhir) (*models.Patient, error) {
	requestJSON, err := json.Marshal(request)
	if err != nil {
		return nil, exceptions.ErrCannotMarshalJSON(err)
	}

	req, err := http.NewRequestWithContext(ctx, constvars.MethodPost, c.BaseUrl, bytes.NewBuffer(requestJSON))
	if err != nil {
		return nil, exceptions.ErrCreateHTTPRequest(err)
	}
	req.Header.Set(constvars.HeaderContentType, constvars.MIMEApplicationFHIRJSON)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, exceptions.ErrSendHTTPRequest(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return nil, exceptions.ErrCreateFHIRResource(nil, constvars.ResourcePatient)
	}

	patientFhir := new(models.Patient)
	err = json.NewDecoder(resp.Body).Decode(&patientFhir)
	if err != nil {
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourcePatient)
	}

	return patientFhir, nil
}

func (c *patientFhirClient) GetPatientByID(ctx context.Context, patientID string) (*models.Patient, error) {
	req, err := http.NewRequestWithContext(ctx, constvars.MethodGet, fmt.Sprintf("%s/%s", c.BaseUrl, patientID), nil)
	if err != nil {
		return nil, exceptions.ErrCreateHTTPRequest(err)
	}
	req.Header.Set(constvars.HeaderContentType, constvars.MIMEApplicationFHIRJSON)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, exceptions.ErrSendHTTPRequest(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != constvars.StatusOK {
		return nil, exceptions.ErrGetFHIRResource(nil, constvars.ResourcePatient)
	}

	patientFhir := new(models.Patient)
	err = json.NewDecoder(resp.Body).Decode(&patientFhir)
	if err != nil {
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourcePatient)
	}

	return patientFhir, nil
}

func (c *patientFhirClient) UpdatePatient(ctx context.Context, request *requests.PatientFhir) (*models.Patient, error) {
	// Convert FHIR Patient to JSON
	requestJSON, err := json.Marshal(request)
	if err != nil {
		return nil, exceptions.ErrCannotMarshalJSON(err)
	}

	// Send PUT request to FHIR server
	req, err := http.NewRequestWithContext(ctx, constvars.MethodPut, fmt.Sprintf("%s/%s", c.BaseUrl, request.ID), bytes.NewBuffer(requestJSON))
	if err != nil {
		return nil, exceptions.ErrCreateHTTPRequest(err)
	}
	req.Header.Set(constvars.HeaderContentType, constvars.MIMEApplicationFHIRJSON)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, exceptions.ErrSendHTTPRequest(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, exceptions.ErrUpdateFHIRResource(nil, constvars.ResourcePatient)
	}

	patientFhir := new(models.Patient)
	err = json.NewDecoder(resp.Body).Decode(&patientFhir)
	if err != nil {
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourcePatient)
	}

	return patientFhir, nil
}
