package appointments

import (
	"context"
	"konsulin-service/internal/pkg/fhir_dto"
)

type AppointmentUsecase interface{}

type AppointmentRepository interface{}

type AppointmentFhirClient interface {
	CreateAppointment(ctx context.Context, request *fhir_dto.Appointment) (*fhir_dto.Appointment, error)
}
