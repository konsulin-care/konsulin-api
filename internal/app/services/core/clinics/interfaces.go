package clinics

import (
	"context"
	"konsulin-service/internal/pkg/dto/responses"
)

type ClinicUsecase interface {
	FindAll(ctx context.Context, page, pageSize int) ([]responses.Clinic, *responses.Pagination, error)
	FindByID(ctx context.Context, clinicID string) (*responses.Clinic, error)
}
