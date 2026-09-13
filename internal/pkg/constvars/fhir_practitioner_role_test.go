package constvars

import "testing"

// TestPractitionerRoleCodeConstants locks the PractitionerRole code constants
// to the FHIR R4 practitioner-role value set:
// https://hl7.org/fhir/R4/valueset-practitioner-role.html
func TestPractitionerRoleCodeConstants(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want string
	}{
		{"researcher code", FhirPractitionerRoleCodeResearcher, "researcher"},
		{"administrative staff code (SNOMED)", FhirPractitionerRoleCodeAdministrativeStaff, "224608005"},
		{"HL7 system", FhirPractitionerRoleSystemHL7, "http://terminology.hl7.org/CodeSystem/practitioner-role"},
		{"SNOMED system", FhirPractitionerRoleSystemSnomed, "http://snomed.info/sct"},
		{"researcher display", FhirPractitionerRoleDisplayResearcher, "Researcher"},
		{"administrative staff display", FhirPractitionerRoleDisplayAdministrativeStaff, "Administrative healthcare staff"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("got %q, want %q", tt.got, tt.want)
			}
		})
	}
}
