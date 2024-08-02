package middlewares

import (
	"context"
	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/services/core/auth"
	"konsulin-service/internal/app/services/core/session"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/utils"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"
)

func NewMiddlewares(logger *zap.Logger, sessionService session.SessionService, authUsecase auth.AuthUsecase, internalConfig *config.InternalConfig) *Middlewares {
	return &Middlewares{
		Log:            logger,
		SessionService: sessionService,
		AuthUsecase:    authUsecase,
		InternalConfig: internalConfig,
	}
}
func (m *Middlewares) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			utils.BuildErrorResponse(m.Log, w, exceptions.ErrTokenMissing(nil))
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")
		sessionID, err := utils.ParseJWT(token, m.InternalConfig.JWT.Secret)
		if err != nil {
			utils.BuildErrorResponse(m.Log, w, exceptions.ErrTokenInvalid(err))
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		sessionData, err := m.SessionService.GetSessionData(ctx, sessionID)
		if err != nil {
			if err == context.DeadlineExceeded {
				utils.BuildErrorResponse(m.Log, w, exceptions.ErrServerDeadlineExceeded(err))
				return
			}
			utils.BuildErrorResponse(m.Log, w, err)
			return
		}

		ctx = context.WithValue(r.Context(), "sessionData", sessionData)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (m *Middlewares) Authorize(resource, requiredAction string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sessionData := r.Context().Value("sessionData").(string)

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			request := requests.AuthorizeUser{
				SessionData:    sessionData,
				Resource:       resource,
				RequiredAction: requiredAction,
			}
			hasPermission, err := m.AuthUsecase.IsUserHasPermission(ctx, request)
			if err != nil {
				if err == context.DeadlineExceeded {
					utils.BuildErrorResponse(m.Log, w, exceptions.ErrServerDeadlineExceeded(err))
					return
				}
				if !hasPermission {
					utils.BuildErrorResponse(m.Log, w, err)
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}
