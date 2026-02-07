package contracts

import (
	"context"
	"konsulin-service/internal/pkg/fhir_dto"
	"net/url"
)

type ScheduleFhirClient interface {
	CreateSchedule(ctx context.Context, request *fhir_dto.Schedule) (*fhir_dto.Schedule, error)
	FindScheduleByPractitionerID(ctx context.Context, practitionerID string) ([]fhir_dto.Schedule, error)
	FindScheduleByPractitionerRoleID(ctx context.Context, practitionerRoleID string) ([]fhir_dto.Schedule, error)
	Search(ctx context.Context, params ScheduleSearchParams) ([]fhir_dto.Schedule, error)
}

// ScheduleSearchParams supports searching by logical id (_id) for now
type ScheduleSearchParams struct {
	ID string
}

func (p ScheduleSearchParams) ToQueryParam() url.Values {
	v := url.Values{}
	if p.ID != "" {
		v.Set("_id", p.ID)
	}
	return v
}
