package clinicians

import (
	"context"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/dto/responses"
)

type ClinicianUsecase interface {
	FindClinicianSummaryByID(ctx context.Context, clinicianID string) (*responses.ClinicianSummary, error)
	CreateAvailibilityTime(ctx context.Context, sessionData string, request *requests.AvailableTime) error
	CreateAppointment(ctx context.Context, sessionData string, request *requests.CreateAppointmentRequest) error
	CreateClinics(ctx context.Context, sessionData string, request *requests.ClinicianCreateClinics) error
	DeleteClinicByID(ctx context.Context, sessionData string, clinicID string) error
}

type ClinicianRepository interface{}
