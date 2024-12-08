package contracts

import (
	"context"
	"konsulin-service/internal/app/models"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/dto/responses"
)

type UserUsecase interface {
	GetUserProfileBySession(ctx context.Context, sessionData string) (*responses.UserProfile, error)
	UpdateUserProfileBySession(ctx context.Context, sessionData string, request *requests.UpdateProfile) (*responses.UpdateUserProfile, error)
	DeleteUserBySession(ctx context.Context, sessionData string) error
	DeactivateUserBySession(ctx context.Context, sessionData string) error
}

type UserRepository interface {
	GetClient(ctx context.Context) (databaseClient interface{})
	CreateUser(ctx context.Context, userModel *models.User) (userID string, err error)
	FindByEmail(ctx context.Context, email string) (*models.User, error)
	FindByUsername(ctx context.Context, username string) (*models.User, error)
	FindByEmailOrUsername(ctx context.Context, email, username string) (*models.User, error)
	FindByWhatsAppNumber(ctx context.Context, whatsAppNumber string) (*models.User, error)
	FindByResetToken(ctx context.Context, token string) (*models.User, error)
	FindByID(ctx context.Context, userID string) (*models.User, error)
	UpdateUser(ctx context.Context, userModel *models.User) error
	DeleteByID(ctx context.Context, email string) error
}
