package webhook

import (
	"context"
	"konsulin-service/internal/app/services/shared/jwtmanager"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/exceptions"
	"strings"

	"go.uber.org/zap"
)

// ExtractAuthContextOutput holds requester identity derived from context.
type ExtractAuthContextOutput struct {
	// IsAPIKey indicates the request used superadmin API key auth.
	IsAPIKey bool
	// UID is the requester user id (or "anonymous" when absent per middleware behavior).
	UID string
	// Roles includes roles assigned by session or API key middleware.
	Roles []string
	// IsSuperadmin is true if roles include superadmin or IsAPIKey is true.
	IsSuperadmin bool
}

// ExtractAuthContext reads values set by APIKeyAuth and SessionOptional middlewares.
// It does not perform any I/O and is safe to call in controllers/handlers.
func ExtractAuthContext(ctx context.Context) *ExtractAuthContextOutput {
	out := &ExtractAuthContextOutput{
		UID:   "",
		Roles: []string{},
	}

	if v := ctx.Value("api_key_auth"); v != nil {
		if b, ok := v.(bool); ok {
			out.IsAPIKey = b
		}
	}

	if v := ctx.Value("uid"); v != nil {
		if s, ok := v.(string); ok {
			out.UID = s
		}
	}

	if v := ctx.Value("roles"); v != nil {
		if list, ok := v.([]string); ok {
			out.Roles = list
		} else if anyList, ok := v.([]interface{}); ok {
			roles := make([]string, 0, len(anyList))
			for _, it := range anyList {
				if s, ok := it.(string); ok {
					roles = append(roles, s)
				}
			}
			out.Roles = roles
		}
	}

	for _, r := range out.Roles {
		if strings.EqualFold(r, constvars.KonsulinRoleSuperadmin) {
			out.IsSuperadmin = true
			break
		}
	}
	if out.IsAPIKey {
		out.IsSuperadmin = true
	}

	return out
}

// EvaluateWebhookAuth returns nil if authorized. It supports forwarded JWT header or falls back to API key/session.
func EvaluateWebhookAuth(ctx context.Context, log *zap.Logger, jwtMgr *jwtmanager.JWTManager, forwardedJWT string) error {
	// Forwarded JWT path
	if strings.TrimSpace(forwardedJWT) != "" {
		out, err := jwtMgr.VerifyToken(ctx, &jwtmanager.VerifyTokenInput{Token: forwardedJWT})
		requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
		if err != nil || out == nil || !out.Valid {
			if log != nil {
				log.Info("webhook auth: forwarded JWT invalid",
					zap.String(constvars.LoggingRequestIDKey, requestID),
					zap.Error(err),
				)
			}
			return exceptions.BuildNewCustomError(nil, constvars.StatusUnauthorized, "Not authorized", "UNAUTHORIZED_WEBHOOK_CALLER")
		}
		if log != nil {
			log.Info("webhook auth: forwarded JWT valid",
				zap.String(constvars.LoggingRequestIDKey, requestID),
			)
		}
		return nil
	}

	// Fallback to API key / session rules
	info := ExtractAuthContext(ctx)
	if info == nil {
		return exceptions.BuildNewCustomError(nil, constvars.StatusUnauthorized, "Not authorized", "UNAUTHORIZED_WEBHOOK_CALLER")
	}
	if info.IsAPIKey || info.IsSuperadmin {
		return nil
	}
	if info.UID != "" && !strings.EqualFold(info.UID, "anonymous") {
		return nil
	}
	return exceptions.BuildNewCustomError(nil, constvars.StatusUnauthorized, "Not authorized", "UNAUTHORIZED_WEBHOOK_CALLER")
}
