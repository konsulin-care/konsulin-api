package appointments

import (
	"context"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/dto/responses"
)

type AppointmentUsecase interface{}

type AppointmentRepository interface{}

type AppointmentFhirClient interface {
	CreateAppointment(ctx context.Context, request *requests.AppointmentFhir) (*responses.Appointment, error)
}
