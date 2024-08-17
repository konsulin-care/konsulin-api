package clinicians

import (
	"context"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/dto/responses"
)

type ClinicianUsecase interface {
	CreateAvailibilityTime(ctx context.Context, sessionData string, request *requests.AvailableTime) error
	CreateAppointment(ctx context.Context, sessionData string, request *requests.CreateAppointmentRequest) error
	CreateClinics(ctx context.Context, sessionData string, request *requests.ClinicianCreateClinics) error
	CreateClinicsAvailability(ctx context.Context, sessionData string, request *requests.CreateClinicsAvailability) error
	DeleteClinicByID(ctx context.Context, sessionData string, clinicID string) error
	FindClinicsByClinicianID(ctx context.Context, clinicianID string) ([]responses.ClinicianClinic, error)
}

type ClinicianRepository interface{}
