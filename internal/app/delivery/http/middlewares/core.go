package middlewares

import (
	"context"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/utils"
	"net/http"
	"time"

	"github.com/supertokens/supertokens-golang/recipe/session/claims"
	"github.com/supertokens/supertokens-golang/recipe/session/sessmodels"
	"github.com/supertokens/supertokens-golang/recipe/userroles/userrolesclaims"
	"github.com/supertokens/supertokens-golang/supertokens"
	"go.uber.org/zap"
)

type responseRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (rec *responseRecorder) WriteHeader(code int) {
	rec.statusCode = code
	rec.ResponseWriter.WriteHeader(code)
}

func (m *Middlewares) Logging(logger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			requestID := r.Context().Value(constvars.CONTEXT_REQUEST_ID_KEY)
			isClientRequestID := r.Context().Value(constvars.CONTEXT_IS_CLIENT_REQUEST_ID_KEY)

			logger.Info("API request started",
				zap.Any(constvars.LoggingRequestIDKey, requestID),
				zap.Any("is_client_request_id", isClientRequestID),
				zap.String(constvars.LoggingMethodKey, r.Method),
				zap.String(constvars.LoggingEndpointKey, r.URL.Path),
				zap.String(constvars.LoggingRemoteAddrKey, r.RemoteAddr),
				zap.String(constvars.LoggingUserAgentKey, r.UserAgent()),
				zap.String(constvars.LoggingQueryKey, r.URL.RawQuery),
			)

			rec := &responseRecorder{ResponseWriter: w, statusCode: http.StatusOK}

			next.ServeHTTP(rec, r)

			logger.Info("API request completed",
				zap.Int(constvars.LoggingStatusCodeKey, rec.statusCode),
				zap.Any(constvars.LoggingRequestIDKey, requestID),
				zap.Any("is_client_request_id", isClientRequestID),
				zap.String(constvars.LoggingMethodKey, r.Method),
				zap.String(constvars.LoggingEndpointKey, r.URL.Path),
				zap.String(constvars.LoggingRemoteAddrKey, r.RemoteAddr),
				zap.String(constvars.LoggingUserAgentKey, r.UserAgent()),
				zap.Duration(constvars.LoggingDurationKey, time.Since(start)),
				zap.Bool(constvars.LoggingSuccessKey, rec.statusCode < 400),
			)
		})
	}
}

func (m *Middlewares) RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get(constvars.HeaderXRequestID)
		isClientRequestID := true

		if requestID == "" {
			requestID = utils.GenerateRequestID()
			isClientRequestID = false
		}

		ctx := context.WithValue(r.Context(), constvars.CONTEXT_REQUEST_ID_KEY, requestID)
		ctx = context.WithValue(ctx, constvars.CONTEXT_IS_CLIENT_REQUEST_ID_KEY, isClientRequestID)

		w.Header().Set(constvars.HeaderXRequestID, requestID)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (m *Middlewares) RequirePermission(method, path string) func([]claims.SessionClaimValidator, sessmodels.SessionContainer, supertokens.UserContext) ([]claims.SessionClaimValidator, error) {
	return func(globalClaimValidators []claims.SessionClaimValidator, sessionContainer sessmodels.SessionContainer, userContext supertokens.UserContext) ([]claims.SessionClaimValidator, error) {
		policies, err := m.Enforcer.GetFilteredPolicy(1, method, path)
		if err != nil {
			return globalClaimValidators, err
		}
		for _, p := range policies {
			if len(p) < 3 {
				continue
			}
			role := p[0]
			globalClaimValidators = append(globalClaimValidators, userrolesclaims.UserRoleClaimValidators.Includes(role, nil, nil))
		}
		return globalClaimValidators, nil
	}
}
