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

// extractAuthContext reads values set by APIKeyAuth and SessionOptional middlewares.
// It does not perform any I/O and is safe to call in controllers/handlers.
func (u *usecase) extractAuthContext(ctx context.Context) *ExtractAuthContextOutput {
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

// evaluateAuthInput encapsulates inputs needed to evaluate webhook auth.
type evaluateAuthInput struct {
	ServiceName  string
	ForwardedJWT string
}

// evaluateWebhookAuth returns nil if authorized. It supports forwarded JWT header or falls back to API key/session.
func (u *usecase) evaluateWebhookAuth(ctx context.Context, in *evaluateAuthInput) error {
	serviceName := in.ServiceName
	forwardedJWT := in.ForwardedJWT
	jwtMgr := u.jwtManager
	log := u.log
	paidOnlyServicesCSV := u.cfg.Webhook.PaidOnlyServices

	// Normalize paid-only services
	mustBeForwarded := false
	if s := strings.TrimSpace(paidOnlyServicesCSV); s != "" {
		for _, it := range strings.Split(s, ",") {
			if strings.EqualFold(strings.TrimSpace(it), serviceName) {
				mustBeForwarded = true
				break
			}
		}
	}
	// Forwarded JWT path
	if strings.TrimSpace(forwardedJWT) != "" {
		out, err := jwtMgr.VerifyToken(ctx, &jwtmanager.VerifyTokenInput{Token: forwardedJWT})
		requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
		if err != nil || out == nil || !out.Valid {
			if log != nil {
				log.Info("webhook auth: forwarded JWT invalid",
					zap.String(constvars.LoggingRequestIDKey, requestID),
				)
			}
			return exceptions.BuildNewCustomError(nil, constvars.StatusUnauthorized, "Not authorized", "UNAUTHORIZED_WEBHOOK_CALLER")
		}
		// Enforce expected subject for payment-originated requests
		if sub, ok := out.Claims["sub"].(string); !ok || !strings.EqualFold(sub, PAYMENT_SERVICE_SUB) {
			if log != nil {
				log.Info("webhook auth: forwarded JWT subject mismatch",
					zap.String(constvars.LoggingRequestIDKey, requestID),
				)
			}
			return exceptions.BuildNewCustomError(nil, constvars.StatusUnauthorized, "Not authorized", "INVALID_AUTH_CLAIM")
		}
		if log != nil {
			log.Info("webhook auth: forwarded JWT valid")
		}
		return nil
	}

	// If service must be forwarded-only and no header provided
	if mustBeForwarded {
		return exceptions.BuildNewCustomError(nil, constvars.StatusPaymentRequired, "payment required to access this service", "PAYMENT_REQUIRED_FOR_SERVICE")
	}

	// Fallback to API key / session rules
	info := u.extractAuthContext(ctx)
	if info == nil {
		return exceptions.BuildNewCustomError(nil, constvars.StatusUnauthorized, "Not authorized", "UNAUTHORIZED_WEBHOOK_CALLER")
	}
	if info.IsAPIKey || info.IsSuperadmin {
		return nil
	}
	// Allow anonymous users as well (uid empty or "anonymous")
	if info.UID == "" || strings.EqualFold(info.UID, "anonymous") {
		return nil
	}
	// Any authenticated uid is allowed
	if info.UID != "" {
		return nil
	}
	return exceptions.BuildNewCustomError(nil, constvars.StatusUnauthorized, "Not authorized", "UNAUTHORIZED_WEBHOOK_CALLER")
}
