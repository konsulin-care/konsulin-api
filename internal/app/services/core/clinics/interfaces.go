package clinics

import (
	"context"
	"konsulin-service/internal/pkg/dto/responses"
)

type ClinicUsecase interface {
	FindAll(ctx context.Context, page, row int) ([]responses.Clinic, *responses.Pagination, error)
}
