package middlewares

import (
	"context"
	"konsulin-service/internal/pkg/constvars"
	"net/http"

	"github.com/supertokens/supertokens-golang/recipe/session"
	"github.com/supertokens/supertokens-golang/recipe/session/sessmodels"
	"go.uber.org/zap"
)

// Deprecated: all context keys must use typed string, such as constvars.ContextKey
const (
	keyFHIRRole                               = "fhirRole"
	keyFHIRID                                 = "fhirID"
	keyRoles                                  = "roles"
	keyUID                                    = "uid"
	supertokenAccessTokenPayloadRolesKey      = "st-role"
	supertokenAccessTokenPayloadRolesValueKey = "v"
)

func (m *Middlewares) SessionOptional(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		if apiKeyAuth, ok := r.Context().Value(ContextAPIKeyAuth).(bool); ok && apiKeyAuth {
			next.ServeHTTP(w, r)
			return
		}

		sessRequired := false
		sess, _ := session.GetSession(r, w, &sessmodels.VerifySessionOptions{SessionRequired: &sessRequired})

		roles := []string{constvars.KonsulinRoleGuest}
		uid := ""

		if sess != nil {
			uid = sess.GetUserID()
			if raw := sess.GetAccessTokenPayload(); raw != nil {
				if rolesData, exists := raw[supertokenAccessTokenPayloadRolesKey]; exists {
					if rolesMap, ok := rolesData.(map[string]interface{}); ok {
						if rolesValue, ok := rolesMap[supertokenAccessTokenPayloadRolesValueKey]; ok {
							if rolesList, ok := rolesValue.([]interface{}); ok {

								roles = []string{}
								for _, item := range rolesList {
									if role, ok := item.(string); ok {
										roles = append(roles, role)
									}
								}
							}
						}
					}
				}
			}
		} else {

			uid = "anonymous"
			roles = []string{constvars.KonsulinRoleGuest}

			m.Log.Info("Anonymous session created",
				zap.String("ip", r.RemoteAddr),
				zap.String("user_agent", r.UserAgent()),
				zap.String("endpoint", r.URL.Path),
				zap.String("method", r.Method),
			)
		}

		ctx := context.WithValue(r.Context(), keyRoles, roles)
		ctx = context.WithValue(ctx, keyUID, uid)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (m *Middlewares) CreateAnonymousSessionIfNeeded(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		if apiKeyAuth, ok := r.Context().Value(ContextAPIKeyAuth).(bool); ok && apiKeyAuth {

			next.ServeHTTP(w, r)
			return
		}

		sessRequired := false
		sess, _ := session.GetSession(r, w, &sessmodels.VerifySessionOptions{SessionRequired: &sessRequired})

		if sess == nil {

			m.Log.Info("Creating anonymous session for request",
				zap.String("ip", r.RemoteAddr),
				zap.String("endpoint", r.URL.Path),
				zap.String("method", r.Method),
			)
		}

		next.ServeHTTP(w, r)
	})
}

func (m *Middlewares) EnsureAnonymousSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		if apiKeyAuth, ok := r.Context().Value(ContextAPIKeyAuth).(bool); ok && apiKeyAuth {

			next.ServeHTTP(w, r)
			return
		}

		sessRequired := false
		sess, _ := session.GetSession(r, w, &sessmodels.VerifySessionOptions{SessionRequired: &sessRequired})

		if sess == nil {

			ctx := context.WithValue(r.Context(), keyRoles, []string{constvars.KonsulinRoleGuest})
			ctx = context.WithValue(ctx, keyUID, "anonymous")

			m.Log.Info("Ensuring anonymous session for request",
				zap.String("ip", r.RemoteAddr),
				zap.String("endpoint", r.URL.Path),
				zap.String("method", r.Method),
			)

			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		next.ServeHTTP(w, r)
	})
}
