package webhook

import (
	"context"
	"konsulin-service/internal/pkg/constvars"
	"testing"

	"github.com/stretchr/testify/assert"
)

// contextWithUID returns a context with a UID value set.
func contextWithUID(ctx context.Context, uid string) context.Context {
	return context.WithValue(ctx, constvars.CONTEXT_UID, uid)
}

// contextWithAPIKeyAuth returns a context with the API key auth flag set.
func contextWithAPIKeyAuth(ctx context.Context, v bool) context.Context {
	return context.WithValue(ctx, constvars.CONTEXT_API_KEY_AUTH, v)
}

// contextWithRoles returns a context with roles set.
func contextWithRoles(ctx context.Context, roles []string) context.Context {
	return context.WithValue(ctx, constvars.CONTEXT_FHIR_ROLE, roles)
}

func TestEvaluateFallbackAuth_AuthenticatedUserAllowed(t *testing.T) {
	// A user with a real UID (non-empty, non-"anonymous") must be allowed.
	ctx := contextWithUID(context.Background(), "user-123")
	err := evaluateFallbackAuth(ctx)
	assert.NoError(t, err, "authenticated user with valid UID should be allowed")
}

func TestEvaluateFallbackAuth_AnonymousUserDenied(t *testing.T) {
	// An anonymous user (UID == "") must be denied.
	ctx := context.Background()
	err := evaluateFallbackAuth(ctx)
	assert.Error(t, err, "anonymous user (empty UID) should be denied")
	if err != nil {
		assert.Contains(t, err.Error(), "UNAUTHORIZED", "error should contain unauthorized dev message")
	}
}

func TestEvaluateFallbackAuth_ExplicitAnonymousDenied(t *testing.T) {
	// An explicit "anonymous" UID must be denied.
	ctx := contextWithUID(context.Background(), "anonymous")
	err := evaluateFallbackAuth(ctx)
	assert.Error(t, err, "explicit anonymous UID should be denied")
}

func TestEvaluateFallbackAuth_APIKeyAlwaysAllowed(t *testing.T) {
	// API key auth bypasses all UID checks.
	ctx := contextWithAPIKeyAuth(context.Background(), true)
	err := evaluateFallbackAuth(ctx)
	assert.NoError(t, err, "API key auth should always be allowed")
}

func TestEvaluateFallbackAuth_SuperadminRoleAllowed(t *testing.T) {
	// Superadmin role bypasses all UID checks.
	ctx := contextWithRoles(context.Background(), []string{constvars.KonsulinRoleSuperadmin})
	err := evaluateFallbackAuth(ctx)
	assert.NoError(t, err, "superadmin role should always be allowed")
}

func TestEvaluateFallbackAuth_AnonymousWithAPIKeyIgnored(t *testing.T) {
	// Even with anonymous UID, API key should still allow.
	ctx := contextWithUID(context.Background(), "anonymous")
	ctx = contextWithAPIKeyAuth(ctx, true)
	err := evaluateFallbackAuth(ctx)
	assert.NoError(t, err, "API key should override anonymous UID")
}
