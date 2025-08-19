package middlewares

import (
	"net/http"
	"time"

	"github.com/go-chi/httprate"
)

// ConditionalRateLimit applies different rate limits based on authentication method
func (m *Middlewares) ConditionalRateLimit(normalLimiter, apiKeyLimiter func(next http.Handler) http.Handler) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if apiKeyAuth, ok := r.Context().Value(ContextAPIKeyAuth).(bool); ok && apiKeyAuth {
				apiKeyLimiter(next).ServeHTTP(w, r)
			} else {
				normalLimiter(next).ServeHTTP(w, r)
			}
		})
	}
}

// CreateRateLimiters creates the rate limiters for normal and API key requests
func (m *Middlewares) CreateRateLimiters() (normalLimiter, apiKeyLimiter func(next http.Handler) http.Handler) {
	normalLimiter = httprate.LimitByIP(m.InternalConfig.App.MaxRequests, time.Second)
	apiKeyLimiter = httprate.LimitByIP(m.InternalConfig.App.SuperadminAPIKeyRateLimit, time.Second)
	return normalLimiter, apiKeyLimiter
}
