package middlewares

import (
	"context"
	"net/http"

	"konsulin-service/internal/pkg/constvars"

	"github.com/supertokens/supertokens-golang/recipe/session"
	"github.com/supertokens/supertokens-golang/recipe/session/sessmodels"
	"go.uber.org/zap"
)

// contextKey is a custom type for context keys to avoid collisions with other packages.
type contextKey string

const (
	keyFHIRRole       contextKey = "fhirRole"
	keyFHIRID         contextKey = "fhirID"
	keyRoles          contextKey = "roles"
	keyUID            contextKey = "uid"
	keyActiveRole     contextKey = "activeRole"
	keyFHIRResourceID contextKey = "fhirResourceId"
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

// buildSessionAuth extracts uid, roles, activeRole and the FHIR resource ID from
// a SuperTokens session.
func buildSessionAuth(sess sessmodels.SessionContainer) (uid string, roles []string, activeRole, fhirResourceID string) {
	uid = sess.GetUserID()
	if raw := sess.GetAccessTokenPayload(); raw != nil {
		roles = extractRolesFromAccessToken(raw)
		if v, ok := raw[constvars.SupertokenPayloadActiveRoleKey].(string); ok && v != "" {
			activeRole = v
		}
		if v, ok := raw[constvars.SupertokenPayloadFhirResourceIDKey].(string); ok {
			fhirResourceID = v
		}
	}
	return
}

// SessionOptional resolves the SuperTokens session when one is present and seeds the
// request context with the caller's uid, roles, active role and FHIR resource ID, under
// both the local keys and the typed constvars keys. Requests without a session continue
// as the anonymous guest; API-key requests pass through untouched.
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
		fhirResourceID := ""

		if sess != nil {
			uid, roles, activeRole, fhirResourceID = buildSessionAuth(sess)
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
		ctx = context.WithValue(ctx, keyFHIRResourceID, fhirResourceID)
		if activeRole != "" {
			ctx = context.WithValue(ctx, keyActiveRole, activeRole)
		}
		ctx = context.WithValue(ctx, constvars.CONTEXT_FHIR_ROLE, roles)
		ctx = context.WithValue(ctx, constvars.CONTEXT_UID, uid)
		ctx = context.WithValue(ctx, constvars.CONTEXT_FHIR_RESOURCE_ID, fhirResourceID)

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

// EnsureAnonymousSession seeds guest auth context (anonymous uid, Guest role, empty FHIR
// resource ID) for requests without a SuperTokens session, publishing the same typed keys
// as SessionOptional. Requests with a session or API-key auth pass through unchanged.
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
			ctx = context.WithValue(ctx, keyFHIRResourceID, "")
			ctx = context.WithValue(ctx, constvars.CONTEXT_FHIR_ROLE, []string{constvars.KonsulinRoleGuest})
			ctx = context.WithValue(ctx, constvars.CONTEXT_UID, "anonymous")
			ctx = context.WithValue(ctx, constvars.CONTEXT_FHIR_RESOURCE_ID, "")

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
