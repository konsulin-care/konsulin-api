package middlewares

import (
	"context"
	"konsulin-service/internal/pkg/constvars"
	"net/http"

	"github.com/supertokens/supertokens-golang/recipe/session"
	"github.com/supertokens/supertokens-golang/recipe/session/sessmodels"
	"go.uber.org/zap"
)

// contextKey is a custom type for context keys to avoid collisions with other packages.
type contextKey string

const (
	keyFHIRRole   contextKey = "fhirRole"
	keyFHIRID     contextKey = "fhirID"
	keyRoles      contextKey = "roles"
	keyUID        contextKey = "uid"
	keyActiveRole contextKey = "activeRole"
)

// extractRolesFromAccessToken extracts the roles list from a SuperTokens access token payload.
func extractRolesFromAccessToken(raw map[string]interface{}) []string {
	rolesData, exists := raw[constvars.SupertokenPayloadRolesKey]
	if !exists {
		return nil
	}
	rolesMap, ok := rolesData.(map[string]interface{})
	if !ok {
		return nil
	}
	rolesValue, ok := rolesMap[constvars.SupertokenPayloadRolesValueKey]
	if !ok {
		return nil
	}
	rolesList, ok := rolesValue.([]interface{})
	if !ok {
		return nil
	}
	roles := make([]string, 0, len(rolesList))
	for _, item := range rolesList {
		if role, ok := item.(string); ok {
			roles = append(roles, role)
		}
	}
	return roles
}

// buildSessionAuth extracts uid, roles and activeRole from a SuperTokens session.
func buildSessionAuth(sess sessmodels.SessionContainer) (uid string, roles []string, activeRole string) {
	uid = sess.GetUserID()
	if raw := sess.GetAccessTokenPayload(); raw != nil {
		roles = extractRolesFromAccessToken(raw)
		if v, ok := raw[constvars.SupertokenPayloadActiveRoleKey].(string); ok && v != "" {
			activeRole = v
		}
	}
	return
}

func (m *Middlewares) SessionOptional(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		if apiKeyAuth, ok := r.Context().Value(ContextAPIKeyAuth).(bool); ok && apiKeyAuth {
			next.ServeHTTP(w, r)
			return
		}

		sessRequired := false
		sess, _ := session.GetSession(r, w, &sessmodels.VerifySessionOptions{SessionRequired: &sessRequired})

		var roles []string
		uid := ""
		activeRole := ""

		if sess != nil {
			uid, roles, activeRole = buildSessionAuth(sess)
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
		if activeRole != "" {
			ctx = context.WithValue(ctx, keyActiveRole, activeRole)
		}
		ctx = context.WithValue(ctx, constvars.CONTEXT_FHIR_ROLE, roles)
		ctx = context.WithValue(ctx, constvars.CONTEXT_UID, uid)

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
			ctx = context.WithValue(ctx, constvars.CONTEXT_FHIR_ROLE, []string{constvars.KonsulinRoleGuest})
			ctx = context.WithValue(ctx, constvars.CONTEXT_UID, "anonymous")

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
