package payments

import (
	"context"
	"testing"

	"konsulin-service/internal/pkg/constvars"
)

// TestDetermineServiceRequestSubject pins the FHIR subject mapping: analyze maps
// to Patient/<id>, other services map to configured Group subjects, case-insensitively.
func TestDetermineServiceRequestSubject(t *testing.T) {
	uc := &paymentUsecase{}
	tests := []struct {
		name      string
		service   string
		patientID string
		want      string
	}{
		{"analyze returns Patient reference", string(constvars.ServiceAnalyze), "pid-1", "Patient/pid-1"},
		{"analyze is case-insensitive", "ANALYZE", "pid-2", "Patient/pid-2"},
		{"report returns practitioner group", string(constvars.ServiceReport), "pid-1", "Group/practitioner"},
		{"performance-report returns clinic admin group", string(constvars.ServicePerformanceReport), "pid-1", "Group/clinic-admin"},
		{"access-dataset returns researcher group", string(constvars.ServiceAccessDataset), "pid-1", "Group/researcher"},
		{"unknown service returns guest group", "unknown", "pid-1", "Group/guest"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := uc.determineServiceRequestSubject(tt.service, tt.patientID); got != tt.want {
				t.Errorf("determineServiceRequestSubject(%q, %q) = %q, want %q", tt.service, tt.patientID, got, tt.want)
			}
		})
	}
}

// TestMapServiceToRequesterResourceType pins the FHIR requester resource type
// mapping: analyze -> Patient, report -> Practitioner, performance-report and
// access-dataset -> Practitioner (clinic admin / researcher resolve via
// Practitioner), unknown -> empty string.
func TestMapServiceToRequesterResourceType(t *testing.T) {
	tests := []struct {
		service string
		want    string
	}{
		{string(constvars.ServiceAnalyze), constvars.ResourcePatient},
		{string(constvars.ServiceReport), constvars.ResourcePractitioner},
		{string(constvars.ServicePerformanceReport), constvars.ResourcePractitioner},
		{string(constvars.ServiceAccessDataset), constvars.ResourcePractitioner},
		{"unknown", ""},
	}
	for _, tt := range tests {
		t.Run(tt.service, func(t *testing.T) {
			if got := mapServiceToRequesterResourceType(tt.service); got != tt.want {
				t.Errorf("mapServiceToRequesterResourceType(%q) = %q, want %q", tt.service, got, tt.want)
			}
		})
	}
}

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
