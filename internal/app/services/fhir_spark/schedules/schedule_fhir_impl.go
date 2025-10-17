package schedules

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
	scheduleFhirClientInstance contracts.ScheduleFhirClient
	onceScheduleFhirClient     sync.Once
)

type scheduleFhirClient struct {
	BaseUrl string
	Log     *zap.Logger
}

func NewScheduleFhirClient(baseUrl string, logger *zap.Logger) contracts.ScheduleFhirClient {
	onceScheduleFhirClient.Do(func() {
		client := &scheduleFhirClient{
			BaseUrl: baseUrl + constvars.ResourceSchedule,
			Log:     logger,
		}
		scheduleFhirClientInstance = client
	})
	return scheduleFhirClientInstance
}

func (c *scheduleFhirClient) CreateSchedule(ctx context.Context, request *fhir_dto.Schedule) (*fhir_dto.Schedule, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	c.Log.Info("scheduleFhirClient.CreateSchedule called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	requestJSON, err := json.Marshal(request)
	if err != nil {
		c.Log.Error("scheduleFhirClient.CreateSchedule error marshaling JSON",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCannotMarshalJSON(err)
	}

	req, err := http.NewRequestWithContext(ctx, constvars.MethodPost, c.BaseUrl, bytes.NewBuffer(requestJSON))
	if err != nil {
		c.Log.Error("scheduleFhirClient.CreateSchedule error creating HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCreateHTTPRequest(err)
	}
	req.Header.Set(constvars.HeaderContentType, constvars.MIMEApplicationFHIRJSON)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.Log.Error("scheduleFhirClient.CreateSchedule error sending HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrSendHTTPRequest(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != constvars.StatusCreated {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			c.Log.Error("scheduleFhirClient.CreateSchedule error reading response body",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, exceptions.ErrCreateFHIRResource(err, constvars.ResourceSchedule)
		}

		var outcome fhir_dto.OperationOutcome
		err = json.Unmarshal(bodyBytes, &outcome)
		if err != nil {
			c.Log.Error("scheduleFhirClient.CreateSchedule error unmarshaling outcome",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, exceptions.ErrCreateFHIRResource(err, constvars.ResourceSchedule)
		}

		if len(outcome.Issue) > 0 {
			fhirErrorIssue := fmt.Errorf(outcome.Issue[0].Diagnostics)
			c.Log.Error("scheduleFhirClient.CreateSchedule FHIR error",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(fhirErrorIssue),
			)
			return nil, exceptions.ErrCreateFHIRResource(fhirErrorIssue, constvars.ResourceSchedule)
		}
	}

	scheduleFhir := new(fhir_dto.Schedule)
	err = json.NewDecoder(resp.Body).Decode(&scheduleFhir)
	if err != nil {
		c.Log.Error("scheduleFhirClient.CreateSchedule error decoding response",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourceSchedule)
	}

	c.Log.Info("scheduleFhirClient.CreateSchedule succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingScheduleIDKey, scheduleFhir.ID),
	)
	return scheduleFhir, nil
}

func (c *scheduleFhirClient) FindScheduleByPractitionerID(ctx context.Context, practitionerID string) ([]fhir_dto.Schedule, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	c.Log.Info("scheduleFhirClient.FindScheduleByPractitionerID called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingPractitionerIDKey, practitionerID),
	)

	req, err := http.NewRequestWithContext(ctx, constvars.MethodGet,
		fmt.Sprintf("%s?actor=Practitioner/%s", c.BaseUrl, practitionerID), nil)
	if err != nil {
		c.Log.Error("scheduleFhirClient.FindScheduleByPractitionerID error creating HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCreateHTTPRequest(err)
	}
	req.Header.Set(constvars.HeaderContentType, constvars.MIMEApplicationFHIRJSON)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.Log.Error("scheduleFhirClient.FindScheduleByPractitionerID error sending HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrSendHTTPRequest(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != constvars.StatusOK {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			c.Log.Error("scheduleFhirClient.FindScheduleByPractitionerID error reading response body",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourceSchedule)
		}
		var outcome fhir_dto.OperationOutcome
		err = json.Unmarshal(bodyBytes, &outcome)
		if err != nil {
			c.Log.Error("scheduleFhirClient.FindScheduleByPractitionerID error unmarshaling outcome",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourceSchedule)
		}
		if len(outcome.Issue) > 0 {
			fhirErrorIssue := fmt.Errorf(outcome.Issue[0].Diagnostics)
			c.Log.Error("scheduleFhirClient.FindScheduleByPractitionerID FHIR error",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(fhirErrorIssue),
			)
			return nil, exceptions.ErrGetFHIRResource(fhirErrorIssue, constvars.ResourceSchedule)
		}
	}

	var result fhir_dto.FHIRBundle
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		c.Log.Error("scheduleFhirClient.FindScheduleByPractitionerID error decoding response",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourceSchedule)
	}

	// Note: Ensure to preallocate slice with zero length if you're appending below.
	schedulesFhir := make([]fhir_dto.Schedule, 0, len(result.Entry))
	for _, entry := range result.Entry {
		var schedule fhir_dto.Schedule
		err := json.Unmarshal(entry.Resource, &schedule)
		if err != nil {
			c.Log.Error("scheduleFhirClient.FindScheduleByPractitionerID error unmarshaling schedule resource",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, exceptions.ErrCannotParseJSON(err)
		}
		schedulesFhir = append(schedulesFhir, schedule)
	}

	c.Log.Info("scheduleFhirClient.FindScheduleByPractitionerID succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Int(constvars.LoggingScheduleCountKey, len(schedulesFhir)),
	)
	return schedulesFhir, nil
}

func (c *scheduleFhirClient) FindScheduleByPractitionerRoleID(ctx context.Context, practitionerRoleID string) ([]fhir_dto.Schedule, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	c.Log.Info("scheduleFhirClient.FindScheduleByPractitionerRoleID called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingPractitionerRoleIDKey, practitionerRoleID),
	)

	req, err := http.NewRequestWithContext(ctx, constvars.MethodGet,
		fmt.Sprintf("%s?actor=PractitionerRole/%s", c.BaseUrl, practitionerRoleID), nil)
	if err != nil {
		c.Log.Error("scheduleFhirClient.FindScheduleByPractitionerRoleID error creating HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCreateHTTPRequest(err)
	}
	req.Header.Set(constvars.HeaderContentType, constvars.MIMEApplicationFHIRJSON)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.Log.Error("scheduleFhirClient.FindScheduleByPractitionerRoleID error sending HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrSendHTTPRequest(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != constvars.StatusOK {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			c.Log.Error("scheduleFhirClient.FindScheduleByPractitionerRoleID error reading response body",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourceSchedule)
		}
		var outcome fhir_dto.OperationOutcome
		err = json.Unmarshal(bodyBytes, &outcome)
		if err != nil {
			c.Log.Error("scheduleFhirClient.FindScheduleByPractitionerRoleID error unmarshaling outcome",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourceSchedule)
		}
		if len(outcome.Issue) > 0 {
			fhirErrorIssue := fmt.Errorf(outcome.Issue[0].Diagnostics)
			c.Log.Error("scheduleFhirClient.FindScheduleByPractitionerRoleID FHIR error",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(fhirErrorIssue),
			)
			return nil, exceptions.ErrGetFHIRResource(fhirErrorIssue, constvars.ResourceSchedule)
		}
	}

	var result struct {
		Total        int    `json:"total"`
		ResourceType string `json:"resourceType"`
		Entry        []struct {
			FullUrl  string            `json:"fullUrl"`
			Resource fhir_dto.Schedule `json:"resource"`
		} `json:"entry"`
	}
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		c.Log.Error("scheduleFhirClient.FindScheduleByPractitionerRoleID error decoding response",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourceSchedule)
	}

	schedulesFhir := make([]fhir_dto.Schedule, len(result.Entry))
	for i, entry := range result.Entry {
		schedulesFhir[i] = entry.Resource
	}

	c.Log.Info("scheduleFhirClient.FindScheduleByPractitionerRoleID succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Int(constvars.LoggingScheduleCountKey, len(schedulesFhir)),
	)
	return schedulesFhir, nil
}
