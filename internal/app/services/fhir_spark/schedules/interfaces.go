package schedules

import (
	"context"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/dto/responses"
)

type ScheduleUsecase interface{}

type ScheduleRepository interface{}

type ScheduleFhirClient interface {
	CreateSchedule(ctx context.Context, request *requests.Schedule) (*responses.Schedule, error)
	FindScheduleByPractitionerID(ctx context.Context, practitionerID string) ([]responses.Schedule, error)
	FindScheduleByPractitionerRoleID(ctx context.Context, practitionerRoleID string) ([]responses.Schedule, error)
}
