package appointments

import (
	"context"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/dto/responses"
)

type AppointmentUsecase interface {
	FindAll(ctx context.Context, sessionData string, queryParamsRequest *requests.QueryParams) ([]responses.Appointment, error)
}

type AppointmentRepository interface{}
