package schedules

import (
	"context"
	"fmt"
	"sync"

	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/app/services/fhir_spark/base"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/fhir_dto"
	"konsulin-service/internal/pkg/fhir_http_client"

	"go.uber.org/zap"
)

var (
	scheduleFhirClientInstance contracts.ScheduleFhirClient
	onceScheduleFhirClient     sync.Once
)

type scheduleFhirClient struct {
	*base.ResourceClient
}

func NewScheduleFhirClient(baseUrl string, logger *zap.Logger) contracts.ScheduleFhirClient {
	onceScheduleFhirClient.Do(func() {
		scheduleFhirClientInstance = &scheduleFhirClient{
			ResourceClient: base.New(baseUrl, constvars.ResourceSchedule, logger),
		}
	})
	return scheduleFhirClientInstance
}

func (c *scheduleFhirClient) CreateSchedule(ctx context.Context, request *fhir_dto.Schedule) (*fhir_dto.Schedule, error) {
	return fhir_http_client.CreateResource(ctx, c.Log, c.Client, c.BaseUrl, request,
		constvars.ResourceSchedule, constvars.LoggingScheduleIDKey)
}

func (c *scheduleFhirClient) FindScheduleByPractitionerID(ctx context.Context, practitionerID string) ([]fhir_dto.Schedule, error) {
	url := fmt.Sprintf("%s?actor=Practitioner/%s", c.BaseUrl, practitionerID)
	return fhir_http_client.SearchResources[fhir_dto.Schedule](ctx, c.Log, c.Client, url,
		constvars.ResourceSchedule)
}

func (c *scheduleFhirClient) FindScheduleByPractitionerRoleID(ctx context.Context, practitionerRoleID string) ([]fhir_dto.Schedule, error) {
	url := fmt.Sprintf("%s?actor=PractitionerRole/%s", c.BaseUrl, practitionerRoleID)
	return fhir_http_client.SearchResources[fhir_dto.Schedule](ctx, c.Log, c.Client, url,
		constvars.ResourceSchedule)
}

func (c *scheduleFhirClient) Search(ctx context.Context, params contracts.ScheduleSearchParams) ([]fhir_dto.Schedule, error) {
	urlStr := c.BaseUrl
	if q := params.ToQueryParam(); q != nil {
		if enc := q.Encode(); enc != "" {
			urlStr += "?" + enc
		}
	}
	return fhir_http_client.SearchResources[fhir_dto.Schedule](ctx, c.Log, c.Client, urlStr,
		constvars.ResourceSchedule)
}
