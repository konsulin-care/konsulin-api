package middlewares

import (
	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/services/core/auth"
	"konsulin-service/internal/app/services/core/session"

	"go.uber.org/zap"
)

type Middlewares struct {
	Log            *zap.Logger
	AuthUsecase    auth.AuthUsecase
	SessionService session.SessionService
	InternalConfig *config.InternalConfig
}

func NewMiddlewares(logger *zap.Logger, sessionService session.SessionService, authUsecase auth.AuthUsecase, internalConfig *config.InternalConfig) *Middlewares {
	return &Middlewares{
		Log:            logger,
		SessionService: sessionService,
		AuthUsecase:    authUsecase,
		InternalConfig: internalConfig,
	}
}
