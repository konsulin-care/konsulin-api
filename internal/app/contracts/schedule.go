package contracts

import (
	"context"
	"konsulin-service/internal/pkg/fhir_dto"
)

type ScheduleFhirClient interface {
	CreateSchedule(ctx context.Context, request *fhir_dto.Schedule) (*fhir_dto.Schedule, error)
	FindScheduleByPractitionerID(ctx context.Context, practitionerID string) ([]fhir_dto.Schedule, error)
	FindScheduleByPractitionerRoleID(ctx context.Context, practitionerRoleID string) ([]fhir_dto.Schedule, error)
}
