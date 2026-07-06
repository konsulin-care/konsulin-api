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

// extractAPIKeyAuth reads the API key auth flag from context.
func extractAPIKeyAuth(ctx context.Context) bool {
	if v := ctx.Value(constvars.CONTEXT_API_KEY_AUTH); v != nil {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}

// extractUID reads the user ID from context.
func extractUID(ctx context.Context) string {
	if v := ctx.Value(constvars.CONTEXT_UID); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// extractRoles reads the FHIR roles from context, supporting both []string and []interface{}.
func extractRoles(ctx context.Context) []string {
	v := ctx.Value(constvars.CONTEXT_FHIR_ROLE)
	if v == nil {
		return nil
	}
	if list, ok := v.([]string); ok {
		return list
	}
	if anyList, ok := v.([]interface{}); ok {
		roles := make([]string, 0, len(anyList))
		for _, it := range anyList {
			if s, ok := it.(string); ok {
				roles = append(roles, s)
			}
		}
		return roles
	}
	return nil
}

// extractIsSuperadmin returns true if any role is superadmin or API key auth is active.
func extractIsSuperadmin(ctx context.Context) bool {
	if extractAPIKeyAuth(ctx) {
		return true
	}
	for _, r := range extractRoles(ctx) {
		if strings.EqualFold(r, constvars.KonsulinRoleSuperadmin) {
			return true
		}
	}
	return false
}

// evaluateAuthInput encapsulates inputs needed to evaluate webhook auth.
type evaluateAuthInput struct {
	ServiceName  string
	ForwardedJWT string
}

// evaluateWebhookAuth returns nil if authorized. It supports forwarded JWT header or falls back to API key/session.
func (u *usecase) evaluateWebhookAuth(ctx context.Context, in *evaluateAuthInput) error {
	if strings.TrimSpace(in.ForwardedJWT) != "" {
		return u.verifyForwardedJWT(ctx, in.ForwardedJWT)
	}

	if isPaidOnlyService(in.ServiceName, u.cfg.Webhook.PaidOnlyServices) {
		return exceptions.BuildNewCustomError(nil, constvars.StatusPaymentRequired, "payment required to access this service", "PAYMENT_REQUIRED_FOR_SERVICE")
	}

	return evaluateFallbackAuth(ctx)
}

// isPaidOnlyService checks whether the given service requires forwarded JWT auth.
func isPaidOnlyService(serviceName, paidOnlyServicesCSV string) bool {
	if s := strings.TrimSpace(paidOnlyServicesCSV); s != "" {
		for _, it := range strings.Split(s, ",") {
			if strings.EqualFold(strings.TrimSpace(it), serviceName) {
				return true
			}
		}
	}
	return false
}

// verifyForwardedJWT validates a forwarded JWT from payment service.
func (u *usecase) verifyForwardedJWT(ctx context.Context, token string) error {
	out, err := u.jwtManager.VerifyToken(ctx, &jwtmanager.VerifyTokenInput{Token: token})
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	if err != nil || out == nil || !out.Valid {
		u.logger(ctx).Info("webhook auth: forwarded JWT invalid",
			zap.String(constvars.LoggingRequestIDKey, requestID),
		)
		return exceptions.BuildNewCustomError(nil, constvars.StatusUnauthorized, "Not authorized", "UNAUTHORIZED_WEBHOOK_CALLER")
	}
	if sub, ok := out.Claims["sub"].(string); !ok || !strings.EqualFold(sub, PAYMENT_SERVICE_SUB) {
		u.logger(ctx).Info("webhook auth: forwarded JWT subject mismatch",
			zap.String(constvars.LoggingRequestIDKey, requestID),
		)
		return exceptions.BuildNewCustomError(nil, constvars.StatusUnauthorized, "Not authorized", "INVALID_AUTH_CLAIM")
	}
	u.logger(ctx).Info("webhook auth: forwarded JWT valid")
	return nil
}

// evaluateFallbackAuth checks API key, superadmin, or any authenticated/anonymous user.
func evaluateFallbackAuth(ctx context.Context) error {
	info := extractAuthContextOutput(ctx)
	if info.IsAPIKey || info.IsSuperadmin {
		return nil
	}
	// Allow anonymous users or any authenticated uid
	if info.UID == "" || strings.EqualFold(info.UID, "anonymous") {
		return nil
	}
	return exceptions.BuildNewCustomError(nil, constvars.StatusUnauthorized, "Not authorized", "UNAUTHORIZED_WEBHOOK_CALLER")
}

// extractAuthContextOutput reads auth values from context into a struct.
func extractAuthContextOutput(ctx context.Context) *ExtractAuthContextOutput {
	return &ExtractAuthContextOutput{
		IsAPIKey:    extractAPIKeyAuth(ctx),
		UID:         extractUID(ctx),
		Roles:       extractRoles(ctx),
		IsSuperadmin: extractIsSuperadmin(ctx),
	}
}

// logger returns the usecase logger or a no-op if nil.
func (u *usecase) logger(ctx context.Context) *zap.Logger {
	if u.log != nil {
		return u.log
	}
	return zap.NewNop()
}
