package middlewares

import (
	"context"
	"konsulin-service/internal/pkg/constvars"
	"net/http"

	"github.com/supertokens/supertokens-golang/recipe/session"
	"github.com/supertokens/supertokens-golang/recipe/session/sessmodels"
)

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
		// Check if API key authentication was already performed
		if apiKeyAuth, ok := r.Context().Value(ContextAPIKeyAuth).(bool); ok && apiKeyAuth {
			// Skip session processing for API key authenticated requests
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
				for _, item := range raw[supertokenAccessTokenPayloadRolesKey].(map[string]interface{})[supertokenAccessTokenPayloadRolesValueKey].([]interface{}) {
					roles = append(roles, item.(string))
				}
			}
		}

		ctx := context.WithValue(r.Context(), keyRoles, roles)
		ctx = context.WithValue(ctx, keyUID, uid)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
