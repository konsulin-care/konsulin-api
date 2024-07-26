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
