package auth

import (
	"context"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/dto/responses"
)

type AuthUsecase interface {
	RegisterUser(ctx context.Context, request *requests.RegisterUser) (*responses.RegisterUser, error)
	LoginUser(ctx context.Context, request *requests.LoginUser) (*responses.LoginUser, error)
	LogoutUser(ctx context.Context, sessionData string) error
}

type AuthRepository interface{}
