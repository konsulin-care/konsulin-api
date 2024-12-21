package fhir_appointments

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/fhir_dto"
	"net/http"
	"time"
)

type appointmentFhirClient struct {
	BaseUrl string
}

func NewAppointmentFhirClient(baseUrl string) contracts.AppointmentFhirClient {
	return &appointmentFhirClient{
		BaseUrl: baseUrl + constvars.ResourceAppointment,
	}
}

func (c *appointmentFhirClient) FindAll(ctx context.Context, queryParamsRequest *requests.QueryParams) ([]fhir_dto.Appointment, error) {
	var queryParams string

	if queryParamsRequest.FetchType == constvars.QueryParamFetchTypeUpcoming {
		queryParams += fmt.Sprintf("_count=1&status=booked&date=ge%s", time.Now().Format(time.DateOnly))
	}

	if queryParamsRequest.PatientID != "" {
		queryParams += fmt.Sprintf("&patient=%s", queryParamsRequest.PatientID)
	}
	if queryParamsRequest.PractitionerID != "" {
		queryParams += fmt.Sprintf("&practitioner=%s", queryParamsRequest.PractitionerID)
	}

	url := fmt.Sprintf("%s?%s", c.BaseUrl, queryParams)

	req, err := http.NewRequestWithContext(ctx, constvars.MethodGet, url, nil)
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
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourceAppointment)
		}

		var outcome fhir_dto.OperationOutcome
		err = json.Unmarshal(bodyBytes, &outcome)
		if err != nil {
			return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourceAppointment)
		}

		if len(outcome.Issue) > 0 {
			fhirErrorIssue := fmt.Errorf(outcome.Issue[0].Diagnostics)
			return nil, exceptions.ErrGetFHIRResource(fhirErrorIssue, constvars.ResourceAppointment)
		}
	}

	var result struct {
		Total int `json:"total"`
		Entry []struct {
			FullUrl  string               `json:"fullUrl"`
			Resource fhir_dto.Appointment `json:"resource"`
		} `json:"entry"`
	}
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourceAppointment)
	}

	appointments := make([]fhir_dto.Appointment, len(result.Entry))
	for i, entry := range result.Entry {
		appointments[i] = entry.Resource
	}

	return appointments, nil
}

func (c *appointmentFhirClient) FindAppointmentByID(ctx context.Context, appointmentID string) (*fhir_dto.Appointment, error) {
	req, err := http.NewRequestWithContext(ctx, constvars.MethodGet, fmt.Sprintf("%s/%s", c.BaseUrl, appointmentID), nil)
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
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourceAppointment)
		}

		var outcome fhir_dto.OperationOutcome
		err = json.Unmarshal(bodyBytes, &outcome)
		if err != nil {
			return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourceAppointment)
		}

		if len(outcome.Issue) > 0 {
			fhirErrorIssue := fmt.Errorf(outcome.Issue[0].Diagnostics)
			return nil, exceptions.ErrGetFHIRResource(fhirErrorIssue, constvars.ResourceAppointment)
		}
	}

	appointmentFhir := new(fhir_dto.Appointment)
	err = json.NewDecoder(resp.Body).Decode(&appointmentFhir)
	if err != nil {
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourceAppointment)
	}

	return appointmentFhir, nil
}

func (c *appointmentFhirClient) CreateAppointment(ctx context.Context, request *fhir_dto.Appointment) (*fhir_dto.Appointment, error) {
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

	if resp.StatusCode != constvars.StatusCreated {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, exceptions.ErrCreateFHIRResource(err, constvars.ResourceAppointment)
		}

		var outcome fhir_dto.OperationOutcome
		err = json.Unmarshal(bodyBytes, &outcome)
		if err != nil {
			return nil, exceptions.ErrCreateFHIRResource(err, constvars.ResourceAppointment)
		}

		if len(outcome.Issue) > 0 {
			fhirErrorIssue := fmt.Errorf(outcome.Issue[0].Diagnostics)
			return nil, exceptions.ErrCreateFHIRResource(fhirErrorIssue, constvars.ResourceAppointment)
		}
	}

	appointmentFhir := new(fhir_dto.Appointment)
	err = json.NewDecoder(resp.Body).Decode(&appointmentFhir)
	if err != nil {
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourceAppointment)
	}

	return appointmentFhir, nil
}

func (c *appointmentFhirClient) UpdateAppointment(ctx context.Context, request *fhir_dto.Appointment) (*fhir_dto.Appointment, error) {
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
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, exceptions.ErrUpdateFHIRResource(err, constvars.ResourceAppointment)
		}

		var outcome fhir_dto.OperationOutcome
		err = json.Unmarshal(bodyBytes, &outcome)
		if err != nil {
			return nil, exceptions.ErrUpdateFHIRResource(err, constvars.ResourceAppointment)
		}

		if len(outcome.Issue) > 0 {
			fhirErrorIssue := fmt.Errorf(outcome.Issue[0].Diagnostics)
			return nil, exceptions.ErrUpdateFHIRResource(fhirErrorIssue, constvars.ResourceAppointment)
		}
	}

	appointmentFhir := new(fhir_dto.Appointment)
	err = json.NewDecoder(resp.Body).Decode(&appointmentFhir)
	if err != nil {
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourceAppointment)
	}

	return appointmentFhir, nil
}
