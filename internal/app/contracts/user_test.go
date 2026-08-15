package contracts

import (
	"testing"

	"konsulin-service/internal/pkg/constvars"

	"github.com/stretchr/testify/assert"
)

func TestInitializeNewUserFHIRResourcesInput_Resources(t *testing.T) {
	prPractitioner := FHIRResourcePlan{ResourceType: constvars.ResourcePractitioner}
	prPatient := FHIRResourcePlan{ResourceType: constvars.ResourcePatient}
	prAdminRole := FHIRResourcePlan{
		ResourceType:  constvars.ResourcePractitionerRole,
		CodingSystem:  constvars.FhirPractitionerRoleSystemSnomed,
		CodingCode:    constvars.FhirPractitionerRoleCodeAdministrativeStaff,
		CodingDisplay: constvars.FhirPractitionerRoleDisplayAdministrativeStaff,
	}
	prResearcherRole := FHIRResourcePlan{
		ResourceType:  constvars.ResourcePractitionerRole,
		CodingSystem:  constvars.FhirPractitionerRoleSystemHL7,
		CodingCode:    constvars.FhirPractitionerRoleCodeResearcher,
		CodingDisplay: constvars.FhirPractitionerRoleDisplayResearcher,
	}

	tests := []struct {
		name  string
		input InitializeNewUserFHIRResourcesInput
		want  []FHIRResourcePlan
	}{
		{
			name:  "no roles",
			input: InitializeNewUserFHIRResourcesInput{},
			want:  []FHIRResourcePlan{},
		},
		{
			name:  "patient only",
			input: InitializeNewUserFHIRResourcesInput{PatientRolesExists: true},
			want:  []FHIRResourcePlan{prPatient},
		},
		{
			name:  "practitioner only",
			input: InitializeNewUserFHIRResourcesInput{PractitionerRolesExists: true},
			want:  []FHIRResourcePlan{prPractitioner},
		},
		{
			name:  "clinic admin",
			input: InitializeNewUserFHIRResourcesInput{ClinicAdminRolesExists: true},
			want:  []FHIRResourcePlan{prPractitioner, prAdminRole},
		},
		{
			name:  "researcher",
			input: InitializeNewUserFHIRResourcesInput{ResearcherRolesExists: true},
			want:  []FHIRResourcePlan{prPractitioner, prResearcherRole},
		},
		{
			name: "clinic admin + researcher dedupes practitioner",
			input: InitializeNewUserFHIRResourcesInput{
				ClinicAdminRolesExists: true,
				ResearcherRolesExists:  true,
			},
			want: []FHIRResourcePlan{prPractitioner, prAdminRole, prResearcherRole},
		},
		{
			name: "patient + practitioner + clinic admin",
			input: InitializeNewUserFHIRResourcesInput{
				PatientRolesExists:      true,
				PractitionerRolesExists: true,
				ClinicAdminRolesExists:  true,
			},
			want: []FHIRResourcePlan{prPractitioner, prPatient, prAdminRole},
		},
		{
			name:  "superadmin only creates nothing",
			input: InitializeNewUserFHIRResourcesInput{},
			want:  []FHIRResourcePlan{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.input.Resources())
		})
	}
}

func TestInitializeNewUserFHIRResourcesInput_ToogleByRoles(t *testing.T) {
	t.Run("maps known roles to toggles", func(t *testing.T) {
		input := InitializeNewUserFHIRResourcesInput{}
		input.ToogleByRoles([]string{
			constvars.KonsulinRolePatient,
			constvars.KonsulinRolePractitioner,
			constvars.KonsulinRoleClinicAdmin,
			constvars.KonsulinRoleResearcher,
		})
		assert.True(t, input.PatientRolesExists)
		assert.True(t, input.PractitionerRolesExists)
		assert.True(t, input.ClinicAdminRolesExists)
		assert.True(t, input.ResearcherRolesExists)
	})

	t.Run("superadmin sets no toggle (no FHIR resource)", func(t *testing.T) {
		input := InitializeNewUserFHIRResourcesInput{}
		input.ToogleByRoles([]string{constvars.KonsulinRoleSuperadmin})
		assert.False(t, input.PatientRolesExists)
		assert.False(t, input.PractitionerRolesExists)
		assert.False(t, input.ClinicAdminRolesExists)
		assert.False(t, input.ResearcherRolesExists)
	})

	t.Run("unknown roles are ignored", func(t *testing.T) {
		input := InitializeNewUserFHIRResourcesInput{}
		input.ToogleByRoles([]string{"Guest", "Unknown"})
		assert.False(t, input.PatientRolesExists)
		assert.False(t, input.PractitionerRolesExists)
		assert.False(t, input.ClinicAdminRolesExists)
		assert.False(t, input.ResearcherRolesExists)
	})
}
