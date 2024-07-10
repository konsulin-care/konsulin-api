package middlewares

import (
	"context"
	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/services/shared/redis"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/utils"
	"net/http"
	"strings"
)

func NewMiddlewares(redisRepository redis.RedisRepository, internalConfig *config.InternalConfig) *Middlewares {
	return &Middlewares{
		RedisRepository: redisRepository,
		InternalConfig:  internalConfig,
	}
}
func (m *Middlewares) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			utils.BuildErrorResponse(w, exceptions.ErrTokenMissing(nil))
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")
		sessionID, err := utils.ParseJWT(token, m.InternalConfig.JWT.Secret)
		if err != nil {
			utils.BuildErrorResponse(w, err)
			return
		}

		sessionData, err := m.RedisRepository.Get(context.Background(), sessionID)
		if err != nil {
			utils.BuildErrorResponse(w, err)
			return
		}

		ctx := context.WithValue(r.Context(), "sessionData", sessionData)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
