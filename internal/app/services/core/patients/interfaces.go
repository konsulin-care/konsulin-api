package patients

import (
	"context"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/fhir_dto"
)

type PatientUsecase interface {
	CreateAppointment(ctx context.Context, sessionData string, request *requests.CreateAppointmentRequest) (*fhir_dto.Appointment, error)
}

type PatientRepository interface{}
