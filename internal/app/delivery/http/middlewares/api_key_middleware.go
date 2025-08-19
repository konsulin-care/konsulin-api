package middlewares

import (
	"context"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/utils"
	"net/http"

	"go.uber.org/zap"
)

const (
	HeaderAPIKey      = "x-api-key"
	ContextAPIKeyAuth = "api_key_auth"
)

func (m *Middlewares) APIKeyAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiKey := r.Header.Get(HeaderAPIKey)

		if apiKey == "" {
			// No API key, continue with normal flow
			next.ServeHTTP(w, r)
			return
		}

		// Validate API key
		if apiKey != m.InternalConfig.App.SuperadminAPIKey {
			utils.BuildErrorResponse(m.Log, w, exceptions.ErrInvalidAPIKey(nil))
			return
		}

		// Set superadmin context
		ctx := context.WithValue(r.Context(), ContextAPIKeyAuth, true)
		ctx = context.WithValue(ctx, keyRoles, []string{constvars.KonsulinRoleSuperadmin})
		ctx = context.WithValue(ctx, keyUID, "api-key-superadmin")

		// Add audit logging
		m.Log.Info("API Key authentication successful",
			zap.String("ip", r.RemoteAddr),
			zap.String("endpoint", r.URL.Path),
			zap.String("method", r.Method),
			zap.String("user_agent", r.UserAgent()))

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
