package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// The ownership classification maps (RequiresPatientOwnership /
// RequiresPractitionerOwnership / IsPublicResource) were removed in favour of
// the declarative spec in internal/pkg/ownership (single source of truth). The
// path helpers below remain the contract of this package.

func TestExtractResourceTypeFromPath(t *testing.T) {
	cases := map[string]string{
		"/fhir/Patient/123":                   "Patient",
		"/Patient/123":                        "Patient",
		"/fhir/Organization?_elements=name":   "Organization",
		"/fhir/Appointment?actor=Patient/123": "Appointment",
		"/fhir/Observation":                   "Observation",
		"/fhir":                               "fhir",
		"":                                    "",
	}
	for path, want := range cases {
		assert.Equal(t, want, ExtractResourceTypeFromPath(path), "ExtractResourceTypeFromPath(%q)", path)
	}
}

func TestNormalizePath(t *testing.T) {
	assert.Equal(t, "/fhir/Patient/123", NormalizePath("Patient/123"))
	assert.Equal(t, "/fhir/Patient/123", NormalizePath("/fhir/Patient/123"))
}

func TestPathMatch(t *testing.T) {
	assert.True(t, PathMatch("/fhir/Patient/123", "/fhir/Patient"))
	assert.True(t, PathMatch("/fhir/Patient", "/fhir/Patient"))
	assert.False(t, PathMatch("/fhir/PatientX", "/fhir/Patient"))
}
