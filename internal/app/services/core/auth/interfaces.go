package auth

import (
	"context"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/dto/responses"
)

type AuthUsecase interface {
	RegisterPatient(ctx context.Context, request *requests.RegisterUser) (*responses.RegisterUser, error)
	RegisterClinician(ctx context.Context, request *requests.RegisterUser) (*responses.RegisterUser, error)
	LoginPatient(ctx context.Context, request *requests.LoginUser) (*responses.LoginUser, error)
	LoginClinician(ctx context.Context, request *requests.LoginUser) (*responses.LoginUser, error)
	LogoutUser(ctx context.Context, sessionData string) error
	GetSessionData(ctx context.Context, sessionID string) (sessionData string, err error)
	IsUserHasPermission(ctx context.Context, request requests.AuthorizeUser) (hasPermission bool, err error)
	ForgotPassword(request *requests.ForgotPassword) error
	ResetPassword(request *requests.ResetPassword) error
}

type AuthRepository interface{}
