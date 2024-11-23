package patients

import (
	"context"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/dto/responses"
)

type PatientUsecase interface {
	CreateAppointment(ctx context.Context, sessionData string, request *requests.CreateAppointmentRequest) (*responses.CreateAppointment, error)
}

type PatientRepository interface{}
