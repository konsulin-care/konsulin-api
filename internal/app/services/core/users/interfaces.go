package users

import (
	"context"
	"konsulin-service/internal/app/models"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/dto/responses"
)

type UserUsecase interface {
	GetUserProfileBySession(ctx context.Context, sessionData string) (*responses.UserProfile, error)
	UpdateUserProfileBySession(ctx context.Context, sessionData string, request *requests.UpdateProfile) (*responses.UpdateUserProfile, error)
}

type UserRepository interface {
	CreateUser(ctx context.Context, userModel *models.User) (userID string, err error)
	FindByEmail(ctx context.Context, email string) (*models.User, error)
	FindByUsername(ctx context.Context, username string) (*models.User, error)
	FindByResetToken(ctx context.Context, token string) (*models.User, error)
	GetUserByID(ctx context.Context, userID string) (*models.User, error)
	UpdateUser(ctx context.Context, userModel *models.User) error
}
