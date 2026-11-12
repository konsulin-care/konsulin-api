package payments

import (
	"context"
	"testing"

	"konsulin-service/internal/pkg/constvars"
)

// TestWhitelistAccessByRoles pins role whitelisting: any role from the context
// that is present in allowedRoles grants access; missing/empty roles deny it.
func TestWhitelistAccessByRoles(t *testing.T) {
	ctxWithRoles := context.WithValue(context.Background(), constvars.CONTEXT_FHIR_ROLE, []string{constvars.KonsulinRolePatient})

	tests := []struct {
		name         string
		ctx          context.Context
		allowedRoles []string
		want         bool
	}{
		{"matching role grants access", ctxWithRoles, []string{constvars.KonsulinRolePatient, constvars.KonsulinRoleSuperadmin}, true},
		{"non-matching role denies access", ctxWithRoles, []string{constvars.KonsulinRoleSuperadmin}, false},
		{"missing context roles deny access", context.Background(), []string{constvars.KonsulinRolePatient}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := whitelistAccessByRoles(tt.ctx, tt.allowedRoles); got != tt.want {
				t.Errorf("whitelistAccessByRoles(%v, %v) = %v, want %v", tt.ctx.Value(constvars.CONTEXT_FHIR_ROLE), tt.allowedRoles, got, tt.want)
			}
		})
	}
}
