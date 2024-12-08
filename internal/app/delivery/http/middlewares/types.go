package middlewares

import (
	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/contracts"

	"go.uber.org/zap"
)

type Middlewares struct {
	Log            *zap.Logger
	AuthUsecase    contracts.AuthUsecase
	SessionService contracts.SessionService
	InternalConfig *config.InternalConfig
}

func NewMiddlewares(logger *zap.Logger, sessionService contracts.SessionService, authUsecase contracts.AuthUsecase, internalConfig *config.InternalConfig) *Middlewares {
	return &Middlewares{
		Log:            logger,
		SessionService: sessionService,
		AuthUsecase:    authUsecase,
		InternalConfig: internalConfig,
	}
}
