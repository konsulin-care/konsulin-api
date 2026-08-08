package utils

import (
	"testing"
)

// TestRequiresPatientOwnership_Communication asserts Communication is classified
// as a patient-owned resource so the response ownership filter keeps patient
// referrals instead of stripping them as practitioner-only.
func TestRequiresPatientOwnership_Communication(t *testing.T) {
	if !RequiresPatientOwnership("Communication") {
		t.Fatal("Communication must require patient ownership")
	}
}

// TestRequiresPractitionerOwnership_Communication keeps the existing
// practitioner classification intact for Communication.
func TestRequiresPractitionerOwnership_Communication(t *testing.T) {
	if !RequiresPractitionerOwnership("Communication") {
		t.Fatal("Communication must require practitioner ownership")
	}
}

// TestRequiresPatientOwnership_Samples guards representative entries of the
// patient-owned map from accidental regressions.
func TestRequiresPatientOwnership_Samples(t *testing.T) {
	for _, resourceType := range []string{"Patient", "Appointment", "Observation", "Condition", "Consent"} {
		if !RequiresPatientOwnership(resourceType) {
			t.Errorf("expected %s to require patient ownership", resourceType)
		}
	}
	for _, resourceType := range []string{"Practitioner", "Schedule", "HealthcareService"} {
		if RequiresPatientOwnership(resourceType) {
			t.Errorf("expected %s NOT to require patient ownership", resourceType)
		}
	}
}
