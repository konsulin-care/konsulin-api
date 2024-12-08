package contracts

import (
	"context"
	"konsulin-service/internal/app/models"
	"konsulin-service/internal/pkg/dto/responses"
)

type EducationLevelUsecase interface {
	FindAll(ctx context.Context) ([]responses.EducationLevel, error)
}

type EducationLevelRepository interface {
	FindAll(ctx context.Context) ([]models.EducationLevel, error)
	FindByID(ctx context.Context, educationLevelID string) (*models.EducationLevel, error)
	FindByCode(ctx context.Context, educationLevelCode string) (*models.EducationLevel, error)
}
