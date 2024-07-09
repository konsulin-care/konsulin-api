package middlewares

import (
	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/services/shared/redis"
)

type Middlewares struct {
	RedisRepository redis.RedisRepository
	InternalConfig  *config.InternalConfig
}
