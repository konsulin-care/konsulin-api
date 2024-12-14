package contracts

import (
	"context"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/dto/responses"
)

type ClinicianUsecase interface {
	CreatePracticeInformation(ctx context.Context, sessionData string, request *requests.CreatePracticeInformation) ([]responses.PracticeInformation, error)
	CreatePracticeAvailability(ctx context.Context, sessionData string, request *requests.CreatePracticeAvailability) ([]responses.PracticeAvailability, error)
	DeleteClinicByID(ctx context.Context, sessionData string, clinicID string) error
	FindClinicsByClinicianID(ctx context.Context, request *requests.FindClinicianByClinicianID) ([]responses.ClinicianClinic, error)
	FindAvailability(ctx context.Context, request *requests.FindAvailability) (*responses.MonthlyAvailabilityResponse, error)
}

type ClinicianRepository interface{}
