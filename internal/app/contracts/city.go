package contracts

import (
	"context"
	"konsulin-service/internal/app/models"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/dto/responses"
)

type CityUsecase interface {
	FindAll(ctx context.Context, queryParamsRequest *requests.QueryParams) ([]responses.City, error)
}

type CityRepository interface {
	FindAll(ctx context.Context) ([]models.City, error)
	FindByID(ctx context.Context, cityID string) (*models.City, error)
}
