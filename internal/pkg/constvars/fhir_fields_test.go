package constvars

import "testing"

// TestFhirFieldPatient guards the FHIR JSON field name constant that the
// middlewares package uses in place of repeated "patient" literals (goconst).
func TestFhirFieldPatient(t *testing.T) {
	if FhirFieldPatient != "patient" {
		t.Fatalf("FhirFieldPatient = %q, want %q", FhirFieldPatient, "patient")
	}
}

// TestResourceConsent guards the FHIR resource type constant that rbac.go uses
// in place of the repeated "Consent" literal (goconst).
func TestResourceConsent(t *testing.T) {
	if ResourceConsent != "Consent" {
		t.Fatalf("ResourceConsent = %q, want %q", ResourceConsent, "Consent")
	}
}
