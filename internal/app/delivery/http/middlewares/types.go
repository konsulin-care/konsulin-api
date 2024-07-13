package middlewares

import (
	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/services/core/auth"
)

type Middlewares struct {
	AuthUsecase    auth.AuthUsecase
	InternalConfig *config.InternalConfig
}
