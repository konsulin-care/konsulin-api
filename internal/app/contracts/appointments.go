package contracts

import (
	"context"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/dto/responses"
	"konsulin-service/internal/pkg/fhir_dto"
)

type AppointmentUsecase interface {
	FindAll(ctx context.Context, sessionData string, queryParamsRequest *requests.QueryParams) ([]responses.Appointment, error)
}

type AppointmentFhirClient interface {
	FindAll(ctx context.Context, queryParamsRequest *requests.QueryParams) ([]fhir_dto.Appointment, error)
	CreateAppointment(ctx context.Context, request *fhir_dto.Appointment) (*fhir_dto.Appointment, error)
}

type AppointmentRepository interface{}
