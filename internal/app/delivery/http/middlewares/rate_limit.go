package middlewares

import (
	"net"
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

// remoteAddrKey derives the rate-limit key from r.RemoteAddr, the TCP peer
// that opened the connection. It is a byte-for-byte clone of the deprecated
// httprate.KeyByIP: SplitHostPort on RemoteAddr (raw fallback), then
// CanonicalizeIP (IPv4 unchanged, IPv6 masked to /64). Kept private so the
// trust model stays explicit while the keying contract is unchanged.
func remoteAddrKey(r *http.Request) (string, error) {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		ip = r.RemoteAddr
	}
	return httprate.CanonicalizeIP(ip), nil
}

// CreateRateLimiters creates the rate limiters for normal and API key requests
func (m *Middlewares) CreateRateLimiters() (normalLimiter, apiKeyLimiter func(next http.Handler) http.Handler) {
	normalLimiter = httprate.LimitBy(m.InternalConfig.App.MaxRequests, time.Second, remoteAddrKey)
	apiKeyLimiter = httprate.LimitBy(m.InternalConfig.App.SuperadminAPIKeyRateLimit, time.Second, remoteAddrKey)
	return normalLimiter, apiKeyLimiter
}
