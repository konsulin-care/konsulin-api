package contracts

import (
	"context"
	"konsulin-service/internal/app/models"
	"konsulin-service/internal/pkg/dto/responses"
)

type GenderUsecase interface {
	FindAll(ctx context.Context) ([]responses.Gender, error)
}

type GenderRepository interface {
	FindAll(ctx context.Context) ([]models.Gender, error)
	FindByID(ctx context.Context, genderID string) (*models.Gender, error)
	FindByCode(ctx context.Context, genderCode string) (*models.Gender, error)
}
