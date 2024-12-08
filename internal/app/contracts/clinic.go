package contracts

import (
	"context"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/dto/responses"
)

type ClinicUsecase interface {
	FindAll(ctx context.Context, nameFilter, fetchType string, page, pageSize int) ([]responses.Clinic, *responses.Pagination, error)
	FindAllCliniciansByClinicID(ctx context.Context, request *requests.FindAllCliniciansByClinicID) ([]responses.ClinicClinician, *responses.Pagination, error)
	FindByID(ctx context.Context, clinicID string) (*responses.Clinic, error)
	FindClinicianByClinicAndClinicianID(ctx context.Context, clinicID, clinicianID string) (*responses.ClinicianSummary, error)
}
