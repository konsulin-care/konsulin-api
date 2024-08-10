package schedules

import (
	"context"
	"konsulin-service/internal/pkg/dto/responses"
)

type ScheduleUsecase interface{}

type ScheduleRepository interface{}

type ScheduleFhirClient interface {
	FindScheduleByPractitionerID(ctx context.Context, practitionerID string) ([]responses.Schedule, error)
}
