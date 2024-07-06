package middlewares

import (
	"context"
	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/services/shared/redis"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/utils"
	"strings"

	"github.com/gofiber/fiber/v2"
)

type Middlewares struct {
	RedisRepository redis.RedisRepository
	InternalConfig  *config.InternalConfig
}

func NewMiddlewares(redisRepository redis.RedisRepository, internalConfig *config.InternalConfig) *Middlewares {
	return &Middlewares{
		RedisRepository: redisRepository,
		InternalConfig:  internalConfig,
	}
}

func (m *Middlewares) AuthMiddleware(c *fiber.Ctx) error {
	authHeader := c.Get("Authorization")
	if authHeader == "" {
		return exceptions.WrapWithoutError(constvars.StatusUnauthorized, constvars.ErrClientNotAuthorized, constvars.ErrDevAuthTokenMissing)
	}

	token := strings.TrimPrefix(authHeader, "Bearer ")
	sessionID, err := utils.ParseJWT(token, m.InternalConfig.JWT.Secret)
	if err != nil {
		return err
	}

	sessionData, err := m.RedisRepository.Get(context.Background(), sessionID)
	if err != nil {
		return exceptions.WrapWithoutError(constvars.StatusUnauthorized, constvars.ErrClientNotAuthorized, constvars.ErrDevAuthInvalidSession)
	}

	c.Locals("sessionData", sessionData)
	c.Locals("sessionID", sessionID)
	return c.Next()
}
