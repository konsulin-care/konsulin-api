package payments

import (
	"context"
	"testing"

	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/fhir_dto"

	"github.com/stretchr/testify/assert"
)

// TestLookupIdentityByService pins identity resolution per service: analyze
// resolves via Patient; report, performance-report, and access-dataset all
// resolve via Practitioner (identical branches merged after the Person identity
// was dropped), and unknown services are rejected.
func TestLookupIdentityByService(t *testing.T) {
	t.Run("practitioner services resolve the practitioner by email", func(t *testing.T) {
		uc := &paymentUsecase{
			PractitionerFhirClient: &mockPractitionerFhirClient{
				byEmail: func(_ context.Context, _ string) ([]fhir_dto.Practitioner, error) {
					return []fhir_dto.Practitioner{{ID: "prac-1", Name: []fhir_dto.HumanName{{Text: "Jane Doe"}}}}, nil
				},
			},
		}
		for _, service := range []string{
			string(constvars.ServiceReport),
			string(constvars.ServicePerformanceReport),
			string(constvars.ServiceAccessDataset),
		} {
			id, name, err := uc.lookupIdentityByService(context.Background(), service, "jane@example.com")
			assert.NoError(t, err, "service %s", service)
			assert.Equal(t, "prac-1", id, "service %s", service)
			assert.Equal(t, "Jane Doe", name, "service %s", service)
		}
	})

	t.Run("practitioner services report not found when no match", func(t *testing.T) {
		uc := &paymentUsecase{PractitionerFhirClient: &mockPractitionerFhirClient{}}
		for _, service := range []string{
			string(constvars.ServiceReport),
			string(constvars.ServicePerformanceReport),
			string(constvars.ServiceAccessDataset),
		} {
			_, _, err := uc.lookupIdentityByService(context.Background(), service, "nobody@example.com")
			assert.Error(t, err, "service %s", service)
			assert.Contains(t, err.Error(), "no practitioner found", "service %s", service)
		}
	})

	t.Run("unsupported service rejected", func(t *testing.T) {
		uc := &paymentUsecase{}
		_, _, err := uc.lookupIdentityByService(context.Background(), "unknown", "x@example.com")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported service")
	})
}

// to Patient/<id>, other services map to configured Group subjects, case-insensitively.
func TestDetermineServiceRequestSubject(t *testing.T) {
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
			if got := determineServiceRequestSubject(tt.service, tt.patientID); got != tt.want {
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
