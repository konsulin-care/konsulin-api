package auth

import (
	"testing"

	"konsulin-service/internal/pkg/constvars"

	"github.com/stretchr/testify/assert"
)

// TestRolesForCreateCode locks the create-code role semantics: existing roles replace
// the default Patient role, which is only assumed for users with no roles yet.
func TestRolesForCreateCode(t *testing.T) {
	tests := []struct {
		name    string
		fetched []string
		want    []string
	}{
		{"first-time user defaults to Patient", nil, []string{constvars.KonsulinRolePatient}},
		{"empty roles default to Patient", []string{}, []string{constvars.KonsulinRolePatient}},
		{"practitioner is not given Patient", []string{constvars.KonsulinRolePractitioner}, []string{constvars.KonsulinRolePractitioner}},
		{"superadmin is not given Patient", []string{constvars.KonsulinRoleSuperadmin}, []string{constvars.KonsulinRoleSuperadmin}},
		{"dual role is kept as-is", []string{constvars.KonsulinRolePatient, constvars.KonsulinRolePractitioner}, []string{constvars.KonsulinRolePatient, constvars.KonsulinRolePractitioner}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, rolesForCreateCode(tt.fetched))
		})
	}
}
