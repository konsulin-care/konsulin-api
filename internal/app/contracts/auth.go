package contracts

import (
	"context"
	"konsulin-service/internal/pkg/dto/requests"
)

type AuthUsecase interface {
	InitializeSupertoken() error
	LogoutUser(ctx context.Context, sessionData string) error
	CreateMagicLink(ctx context.Context, request *requests.SupertokenPasswordlessCreateMagicLink) error
	CreateAnonymousSession(ctx context.Context) (string, error)
}

type AuthRepository interface{}
