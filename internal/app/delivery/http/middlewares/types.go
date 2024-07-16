package middlewares

import (
	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/services/core/auth"

	"go.uber.org/zap"
)

type Middlewares struct {
	Log            *zap.Logger
	AuthUsecase    auth.AuthUsecase
	InternalConfig *config.InternalConfig
}
