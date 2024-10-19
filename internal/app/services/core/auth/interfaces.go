package auth

import (
	"context"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/dto/responses"
)

type AuthUsecase interface {
	LoginViaWhatsApp(ctx context.Context, request *requests.LoginViaWhatsApp) error
	VerifyWhatsAppOTP(ctx context.Context, request *requests.VerivyWhatsAppOTP) (*responses.LoginUser, error)
	RegisterPatient(ctx context.Context, request *requests.RegisterUser) (*responses.RegisterUser, error)
	RegisterClinician(ctx context.Context, request *requests.RegisterUser) (*responses.RegisterUser, error)
	LoginPatient(ctx context.Context, request *requests.LoginUser) (*responses.LoginUser, error)
	LoginClinician(ctx context.Context, request *requests.LoginUser) (*responses.LoginUser, error)
	LogoutUser(ctx context.Context, sessionData string) error
	IsUserHasPermission(ctx context.Context, request requests.AuthorizeUser) (hasPermission bool, err error)
	ForgotPassword(ctx context.Context, request *requests.ForgotPassword) error
	ResetPassword(ctx context.Context, request *requests.ResetPassword) error
}

type AuthRepository interface{}
