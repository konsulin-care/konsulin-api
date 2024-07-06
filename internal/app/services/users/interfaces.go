package users

import (
	"context"
	"konsulin-service/internal/app/models"
)

type UserUsecase interface{}

type UserRepository interface {
	CreateUser(ctx context.Context, userEntity *models.User) (userID string, err error)
	FindByEmail(ctx context.Context, email string) (*models.User, error)
	FindByUsername(ctx context.Context, username string) (*models.User, error)
	GetUserByID(ctx context.Context, userID string) (*models.User, error)
	UpdateUser(ctx context.Context, userID string, updateData map[string]interface{}) error
}

type UserFhirClient interface{}
