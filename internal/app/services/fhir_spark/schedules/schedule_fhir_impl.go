package schedules

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
	scheduleFhirClientInstance contracts.ScheduleFhirClient
	onceScheduleFhirClient     sync.Once
)

type scheduleFhirClient struct {
	BaseUrl string
	Log     *zap.Logger
	client  *fhir_http_client.FHIRHTTPClient
}

func NewScheduleFhirClient(baseUrl string, logger *zap.Logger) contracts.ScheduleFhirClient {
	onceScheduleFhirClient.Do(func() {
		client := &scheduleFhirClient{
			BaseUrl: baseUrl + constvars.ResourceSchedule,
			Log:     logger,
			client:  fhir_http_client.New(logger),
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

	respBody, err := c.client.Do(ctx, constvars.MethodPost, c.BaseUrl, bytes.NewBuffer(requestJSON))
	if err != nil {
		c.Log.Error("scheduleFhirClient.CreateSchedule FHIR error",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCreateFHIRResource(err, constvars.ResourceSchedule)
	}

	var schedule fhir_dto.Schedule
	if err := json.Unmarshal(respBody, &schedule); err != nil {
		c.Log.Error("scheduleFhirClient.CreateSchedule error decoding response",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourceSchedule)
	}

	c.Log.Info("scheduleFhirClient.CreateSchedule succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingScheduleIDKey, schedule.ID),
	)
	return &schedule, nil
}

func (c *scheduleFhirClient) FindScheduleByPractitionerID(ctx context.Context, practitionerID string) ([]fhir_dto.Schedule, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	c.Log.Info("scheduleFhirClient.FindScheduleByPractitionerID called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingPractitionerIDKey, practitionerID),
	)

	respBody, err := c.client.Do(ctx, constvars.MethodGet,
		fmt.Sprintf("%s?actor=Practitioner/%s", c.BaseUrl, practitionerID), nil)
	if err != nil {
		c.Log.Error("scheduleFhirClient.FindScheduleByPractitionerID FHIR error",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourceSchedule)
	}

	var result fhir_dto.FHIRBundle
	if err := json.Unmarshal(respBody, &result); err != nil {
		c.Log.Error("scheduleFhirClient.FindScheduleByPractitionerID error decoding response",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourceSchedule)
	}

	schedulesFhir := make([]fhir_dto.Schedule, 0, len(result.Entry))
	for _, entry := range result.Entry {
		var schedule fhir_dto.Schedule
		if err := json.Unmarshal(entry.Resource, &schedule); err != nil {
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

	respBody, err := c.client.Do(ctx, constvars.MethodGet,
		fmt.Sprintf("%s?actor=PractitionerRole/%s", c.BaseUrl, practitionerRoleID), nil)
	if err != nil {
		c.Log.Error("scheduleFhirClient.FindScheduleByPractitionerRoleID FHIR error",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourceSchedule)
	}

	var result struct {
		Total        int    `json:"total"`
		ResourceType string `json:"resourceType"`
		Entry        []struct {
			FullUrl  string            `json:"fullUrl"`
			Resource fhir_dto.Schedule `json:"resource"`
		} `json:"entry"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
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

func (c *scheduleFhirClient) Search(ctx context.Context, params contracts.ScheduleSearchParams) ([]fhir_dto.Schedule, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	c.Log.Info("scheduleFhirClient.Search called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	q := params.ToQueryParam()
	if q == nil {
		q = url.Values{}
	}
	urlStr := c.BaseUrl
	if enc := q.Encode(); enc != "" {
		urlStr += "?" + enc
	}

	respBody, err := c.client.Do(ctx, constvars.MethodGet, urlStr, nil)
	if err != nil {
		c.Log.Error("scheduleFhirClient.Search FHIR error",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourceSchedule)
	}

	var bundle fhir_dto.FHIRBundle
	if err := json.Unmarshal(respBody, &bundle); err != nil {
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourceSchedule)
	}

	schedules := make([]fhir_dto.Schedule, 0, len(bundle.Entry))
	for _, e := range bundle.Entry {
		var s fhir_dto.Schedule
		if err := json.Unmarshal(e.Resource, &s); err != nil {
			return nil, exceptions.ErrCannotParseJSON(err)
		}
		schedules = append(schedules, s)
	}

	c.Log.Info("scheduleFhirClient.Search succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Int(constvars.LoggingScheduleCountKey, len(schedules)),
	)
	return schedules, nil
}
