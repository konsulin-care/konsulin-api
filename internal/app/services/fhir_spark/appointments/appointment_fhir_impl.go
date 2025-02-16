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
	"sync"
	"time"

	"go.uber.org/zap"
)

var (
	appointmentFhirClientInstance contracts.AppointmentFhirClient
	onceAppointmentFhirClient     sync.Once
)

type appointmentFhirClient struct {
	BaseUrl string
	Log     *zap.Logger
}

func NewAppointmentFhirClient(baseUrl string, logger *zap.Logger) contracts.AppointmentFhirClient {
	onceAppointmentFhirClient.Do(func() {
		client := &appointmentFhirClient{
			BaseUrl: baseUrl + constvars.ResourceAppointment,
			Log:     logger,
		}
		appointmentFhirClientInstance = client
	})
	return appointmentFhirClientInstance
}

func (c *appointmentFhirClient) FindAll(ctx context.Context, queryParamsRequest *requests.QueryParams) ([]fhir_dto.Appointment, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	c.Log.Info("appointmentFhirClient.FindAll called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Any(constvars.LoggingQueryParamsKey, queryParamsRequest),
	)

	var queryParams string

	if queryParamsRequest.AppointmentStatus == "" {
		queryParamsRequest.AppointmentStatus = constvars.FhirAppointmentStatusBooked
	}

	if queryParamsRequest.FetchType == constvars.QueryParamFetchTypeUpcoming {
		queryParams += fmt.Sprintf("_count=1&status=%s&date=ge%s",
			queryParamsRequest.AppointmentStatus,
			time.Now().Format(time.DateOnly),
		)
	}

	if queryParamsRequest.PatientID != "" {
		queryParams += fmt.Sprintf("&patient=%s", queryParamsRequest.PatientID)
	}
	if queryParamsRequest.PractitionerID != "" {
		queryParams += fmt.Sprintf("&practitioner=%s", queryParamsRequest.PractitionerID)
	}

	url := fmt.Sprintf("%s?%s", c.BaseUrl, queryParams)
	c.Log.Info("appointmentFhirClient.FindAll built URL",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingFhirUrlKey, url),
	)

	req, err := http.NewRequestWithContext(ctx, constvars.MethodGet, url, nil)
	if err != nil {
		c.Log.Error("appointmentFhirClient.FindAll error creating HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCreateHTTPRequest(err)
	}
	req.Header.Set(constvars.HeaderContentType, constvars.MIMEApplicationFHIRJSON)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.Log.Error("appointmentFhirClient.FindAll error sending HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrSendHTTPRequest(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != constvars.StatusOK {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			c.Log.Error("appointmentFhirClient.FindAll error reading response body",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourceAppointment)
		}

		var outcome fhir_dto.OperationOutcome
		err = json.Unmarshal(bodyBytes, &outcome)
		if err != nil {
			c.Log.Error("appointmentFhirClient.FindAll error unmarshaling outcome",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourceAppointment)
		}

		if len(outcome.Issue) > 0 {
			fhirErrorIssue := fmt.Errorf(outcome.Issue[0].Diagnostics)
			c.Log.Error("appointmentFhirClient.FindAll FHIR error",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(fhirErrorIssue),
			)
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
		c.Log.Error("appointmentFhirClient.FindAll error decoding response",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourceAppointment)
	}

	appointments := make([]fhir_dto.Appointment, len(result.Entry))
	for i, entry := range result.Entry {
		appointments[i] = entry.Resource
	}

	c.Log.Info("appointmentFhirClient.FindAll succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Int(constvars.LoggingAppointmentCountKey, len(appointments)),
	)
	return appointments, nil
}

func (c *appointmentFhirClient) FindAppointmentByID(ctx context.Context, appointmentID string) (*fhir_dto.Appointment, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	c.Log.Info("appointmentFhirClient.FindAppointmentByID called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingAppointmentIDKey, appointmentID),
	)

	req, err := http.NewRequestWithContext(ctx, constvars.MethodGet, fmt.Sprintf("%s/%s", c.BaseUrl, appointmentID), nil)
	if err != nil {
		c.Log.Error("appointmentFhirClient.FindAppointmentByID error creating HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCreateHTTPRequest(err)
	}
	req.Header.Set(constvars.HeaderContentType, constvars.MIMEApplicationFHIRJSON)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.Log.Error("appointmentFhirClient.FindAppointmentByID error sending HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrSendHTTPRequest(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != constvars.StatusOK {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			c.Log.Error("appointmentFhirClient.FindAppointmentByID error reading response body",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourceAppointment)
		}

		var outcome fhir_dto.OperationOutcome
		err = json.Unmarshal(bodyBytes, &outcome)
		if err != nil {
			c.Log.Error("appointmentFhirClient.FindAppointmentByID error unmarshaling outcome",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourceAppointment)
		}

		if len(outcome.Issue) > 0 {
			fhirErrorIssue := fmt.Errorf(outcome.Issue[0].Diagnostics)
			c.Log.Error("appointmentFhirClient.FindAppointmentByID FHIR error",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(fhirErrorIssue),
			)
			return nil, exceptions.ErrGetFHIRResource(fhirErrorIssue, constvars.ResourceAppointment)
		}
	}

	appointmentFhir := new(fhir_dto.Appointment)
	err = json.NewDecoder(resp.Body).Decode(&appointmentFhir)
	if err != nil {
		c.Log.Error("appointmentFhirClient.FindAppointmentByID error decoding response",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourceAppointment)
	}

	c.Log.Info("appointmentFhirClient.FindAppointmentByID succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingAppointmentIDKey, appointmentFhir.ID),
	)
	return appointmentFhir, nil
}

func (c *appointmentFhirClient) CreateAppointment(ctx context.Context, request *fhir_dto.Appointment) (*fhir_dto.Appointment, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	c.Log.Info("appointmentFhirClient.CreateAppointment called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	requestJSON, err := json.Marshal(request)
	if err != nil {
		c.Log.Error("appointmentFhirClient.CreateAppointment error marshaling JSON",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCannotMarshalJSON(err)
	}

	req, err := http.NewRequestWithContext(ctx, constvars.MethodPost, c.BaseUrl, bytes.NewBuffer(requestJSON))
	if err != nil {
		c.Log.Error("appointmentFhirClient.CreateAppointment error creating HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCreateHTTPRequest(err)
	}
	req.Header.Set(constvars.HeaderContentType, constvars.MIMEApplicationFHIRJSON)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.Log.Error("appointmentFhirClient.CreateAppointment error sending HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrSendHTTPRequest(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != constvars.StatusCreated {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			c.Log.Error("appointmentFhirClient.CreateAppointment error reading response body",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, exceptions.ErrCreateFHIRResource(err, constvars.ResourceAppointment)
		}

		var outcome fhir_dto.OperationOutcome
		err = json.Unmarshal(bodyBytes, &outcome)
		if err != nil {
			c.Log.Error("appointmentFhirClient.CreateAppointment error unmarshaling outcome",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, exceptions.ErrCreateFHIRResource(err, constvars.ResourceAppointment)
		}

		if len(outcome.Issue) > 0 {
			fhirErrorIssue := fmt.Errorf(outcome.Issue[0].Diagnostics)
			c.Log.Error("appointmentFhirClient.CreateAppointment FHIR error",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(fhirErrorIssue),
			)
			return nil, exceptions.ErrCreateFHIRResource(fhirErrorIssue, constvars.ResourceAppointment)
		}
	}

	appointmentFhir := new(fhir_dto.Appointment)
	err = json.NewDecoder(resp.Body).Decode(&appointmentFhir)
	if err != nil {
		c.Log.Error("appointmentFhirClient.CreateAppointment error decoding response",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourceAppointment)
	}

	c.Log.Info("appointmentFhirClient.CreateAppointment succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingAppointmentIDKey, appointmentFhir.ID),
	)
	return appointmentFhir, nil
}

func (c *appointmentFhirClient) UpdateAppointment(ctx context.Context, request *fhir_dto.Appointment) (*fhir_dto.Appointment, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	c.Log.Info("appointmentFhirClient.UpdateAppointment called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	requestJSON, err := json.Marshal(request)
	if err != nil {
		c.Log.Error("appointmentFhirClient.UpdateAppointment error marshaling JSON",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCannotMarshalJSON(err)
	}

	req, err := http.NewRequestWithContext(ctx, constvars.MethodPut, fmt.Sprintf("%s/%s", c.BaseUrl, request.ID), bytes.NewBuffer(requestJSON))
	if err != nil {
		c.Log.Error("appointmentFhirClient.UpdateAppointment error creating HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCreateHTTPRequest(err)
	}
	req.Header.Set(constvars.HeaderContentType, constvars.MIMEApplicationFHIRJSON)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.Log.Error("appointmentFhirClient.UpdateAppointment error sending HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrSendHTTPRequest(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			c.Log.Error("appointmentFhirClient.UpdateAppointment error reading response body",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, exceptions.ErrUpdateFHIRResource(err, constvars.ResourceAppointment)
		}

		var outcome fhir_dto.OperationOutcome
		err = json.Unmarshal(bodyBytes, &outcome)
		if err != nil {
			c.Log.Error("appointmentFhirClient.UpdateAppointment error unmarshaling outcome",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, exceptions.ErrUpdateFHIRResource(err, constvars.ResourceAppointment)
		}

		if len(outcome.Issue) > 0 {
			fhirErrorIssue := fmt.Errorf(outcome.Issue[0].Diagnostics)
			c.Log.Error("appointmentFhirClient.UpdateAppointment FHIR error",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(fhirErrorIssue),
			)
			return nil, exceptions.ErrUpdateFHIRResource(fhirErrorIssue, constvars.ResourceAppointment)
		}
	}

	appointmentFhir := new(fhir_dto.Appointment)
	err = json.NewDecoder(resp.Body).Decode(&appointmentFhir)
	if err != nil {
		c.Log.Error("appointmentFhirClient.UpdateAppointment error decoding response",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourceAppointment)
	}

	c.Log.Info("appointmentFhirClient.UpdateAppointment succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingAppointmentIDKey, appointmentFhir.ID),
	)
	return appointmentFhir, nil
}
