package roles

import (
	"testing"

	"konsulin-service/internal/pkg/utils"

	"github.com/casbin/casbin/v2"
	"github.com/stretchr/testify/assert"
)

func TestRBACIntegration(t *testing.T) {
	enforcer, err := casbin.NewEnforcer("../../../../../resources/rbac_model.conf", "../../../../../resources/rbac_policy.csv")
	if err != nil {
		t.Skipf("Skipping test due to missing RBAC files: %v", err)
		return
	}

	enforcer.AddFunction("pathMatch", func(args ...interface{}) (interface{}, error) {
		if len(args) != 2 {
			return false, nil
		}
		requestPath, ok1 := args[0].(string)
		policyPath, ok2 := args[1].(string)
		if !ok1 || !ok2 {
			return false, nil
		}
		return utils.PathMatch(requestPath, policyPath), nil
	})

	t.Run("Guest Role Specific Use Cases", func(t *testing.T) {

		allowed, err := enforcer.Enforce("Guest", "GET", "/fhir/Organization?_elements=name,address")
		assert.NoError(t, err)
		assert.True(t, allowed, "Guest should be able to access /fhir/Organization?_elements=name,address")

		allowed, err = enforcer.Enforce("Guest", "GET", "/fhir/Slot?schedule.actor=PractitionerRole/123&start=2025-01-01")
		assert.NoError(t, err)
		assert.True(t, allowed, "Guest should be able to access /fhir/Slot?schedule.actor=PractitionerRole/123&start=2025-01-01")

		allowed, err = enforcer.Enforce("Guest", "GET", "/fhir/Slot?schedule.actor=PractitionerRole/456")
		assert.NoError(t, err)
		assert.True(t, allowed, "Guest should be able to access /fhir/Slot?schedule.actor=PractitionerRole/456")
	})

	t.Run("Clinic Admin Role Specific Use Cases", func(t *testing.T) {

		allowed, err := enforcer.Enforce("Clinic Admin", "GET", "/fhir/Organization?_elements=name,address")
		assert.NoError(t, err)
		assert.True(t, allowed, "Clinic Admin should be able to access /fhir/Organization?_elements=name,address")

		allowed, err = enforcer.Enforce("Clinic Admin", "GET", "/fhir/Slot?schedule.actor=PractitionerRole/789&start=2025-01-01")
		assert.NoError(t, err)
		assert.True(t, allowed, "Clinic Admin should be able to access /fhir/Slot?schedule.actor=PractitionerRole/789&start=2025-01-01")
	})

	t.Run("Superadmin Role Specific Use Cases", func(t *testing.T) {

		allowed, err := enforcer.Enforce("Superadmin", "GET", "/fhir/Organization?_elements=name,address")
		assert.NoError(t, err)
		assert.True(t, allowed, "Superadmin should be able to access /fhir/Organization?_elements=name,address")

		allowed, err = enforcer.Enforce("Superadmin", "GET", "/fhir/Slot?schedule.actor=PractitionerRole/999&start=2025-01-01")
		assert.NoError(t, err)
		assert.True(t, allowed, "Superadmin should be able to access /fhir/Slot?schedule.actor=PractitionerRole/999&start=2025-01-01")
	})

	t.Run("Patient Role Public Resources", func(t *testing.T) {

		allowed, err := enforcer.Enforce("Patient", "GET", "/fhir/Questionnaire?_elements=title,description&subject-type=Person,Patient&status=active&context=popular")
		assert.NoError(t, err)
		assert.True(t, allowed, "Patient should be able to access public questionnaires")

		allowed, err = enforcer.Enforce("Patient", "GET", "/fhir/ResearchStudy?date=ge2025-04-14&_revinclude=List:item")
		assert.NoError(t, err)
		assert.True(t, allowed, "Patient should be able to access public research studies")

		allowed, err = enforcer.Enforce("Patient", "GET", "/fhir/Organization?_elements=name,address")
		assert.NoError(t, err)
		assert.True(t, allowed, "Patient should be able to access public organization info")

		allowed, err = enforcer.Enforce("Patient", "GET", "/fhir/PractitionerRole?active=true")
		assert.NoError(t, err)
		assert.True(t, allowed, "Patient should be able to access public practitioner roles")

		allowed, err = enforcer.Enforce("Patient", "GET", "/fhir/Slot?status=free")
		assert.NoError(t, err)
		assert.True(t, allowed, "Patient should be able to access public slots")
	})

	t.Run("Patient Role Protected Resources", func(t *testing.T) {

		allowed, err := enforcer.Enforce("Patient", "GET", "/fhir/Patient/patient-123")
		assert.NoError(t, err)
		assert.True(t, allowed, "Patient should be able to access their own patient resource")

		allowed, err = enforcer.Enforce("Patient", "GET", "/fhir/Patient/other-patient-456")
		assert.NoError(t, err)
		assert.False(t, allowed, "Patient should not be able to access other patients' resources")

		allowed, err = enforcer.Enforce("Patient", "GET", "/fhir/Appointment?actor=Patient/patient-123")
		assert.NoError(t, err)
		assert.True(t, allowed, "Patient should be able to access their own appointments")

		allowed, err = enforcer.Enforce("Patient", "GET", "/fhir/Appointment?actor=Patient/other-patient-456")
		assert.NoError(t, err)
		assert.False(t, allowed, "Patient should not be able to access other patients' appointments")

		allowed, err = enforcer.Enforce("Patient", "GET", "/fhir/Observation?subject=Patient/patient-123")
		assert.NoError(t, err)
		assert.True(t, allowed, "Patient should be able to access their own observations")

		allowed, err = enforcer.Enforce("Patient", "GET", "/fhir/Observation?subject=Patient/other-patient-456")
		assert.NoError(t, err)
		assert.False(t, allowed, "Patient should not be able to access other patients' observations")
	})

	t.Run("Patient Role Complex Appointment Queries", func(t *testing.T) {

		allowed, err := enforcer.Enforce("Patient", "GET", "/fhir/Appointment?actor=Patient/patient-123&slot.start=ge2025-01-01T00:00:00+00:00&_include=Appointment:actor:PractitionerRole&_include:iterate=PractitionerRole:practitioner&_include=Appointment:slot")
		assert.NoError(t, err)
		assert.True(t, allowed, "Patient should be able to access their own appointments with complex query")

		allowed, err = enforcer.Enforce("Patient", "GET", "/fhir/Appointment?actor=Patient/other-patient-456&slot.start=ge2025-01-01T00:00:00+00:00&_include=Appointment:actor:PractitionerRole&_include:iterate=PractitionerRole:practitioner&_include=Appointment:slot")
		assert.NoError(t, err)
		assert.False(t, allowed, "Patient should not be able to access other patients' appointments with complex query")
	})

	t.Run("Query Parameter Variations", func(t *testing.T) {

		testCases := []struct {
			name     string
			path     string
			expected bool
		}{
			{
				name:     "Basic organization access",
				path:     "/fhir/Organization",
				expected: true,
			},
			{
				name:     "Organization with elements",
				path:     "/fhir/Organization?_elements=name,address",
				expected: true,
			},
			{
				name:     "Organization with different elements",
				path:     "/fhir/Organization?_elements=title,description",
				expected: true,
			},
			{
				name:     "Organization with multiple parameters",
				path:     "/fhir/Organization?_elements=name,address&status=active&_count=10",
				expected: true,
			},
			{
				name:     "Slot with schedule actor",
				path:     "/fhir/Slot?schedule.actor=PractitionerRole/123",
				expected: true,
			},
			{
				name:     "Slot with schedule actor and start date",
				path:     "/fhir/Slot?schedule.actor=PractitionerRole/123&start=2025-01-01",
				expected: true,
			},
			{
				name:     "Slot with multiple parameters",
				path:     "/fhir/Slot?schedule.actor=PractitionerRole/123&start=2025-01-01&status=free&_count=20",
				expected: true,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				allowed, err := enforcer.Enforce("Guest", "GET", tc.path)
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, allowed, "Path: %s", tc.path)
			})
		}
	})

	t.Run("PathMatch Function", func(t *testing.T) {

		testCases := []struct {
			name        string
			requestPath string
			policyPath  string
			expected    bool
		}{
			{
				name:        "Exact match without query",
				requestPath: "/fhir/Organization",
				policyPath:  "/fhir/Organization",
				expected:    true,
			},
			{
				name:        "Base path match with query",
				requestPath: "/fhir/Organization?_elements=name,address",
				policyPath:  "/fhir/Organization",
				expected:    true,
			},
			{
				name:        "Different base paths",
				requestPath: "/fhir/Patient",
				policyPath:  "/fhir/Organization",
				expected:    false,
			},
			{
				name:        "Path with special characters",
				requestPath: "/fhir/Slot?schedule.actor=PractitionerRole/123&start=2025-01-01",
				policyPath:  "/fhir/Slot",
				expected:    true,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				result := utils.PathMatch(tc.requestPath, tc.policyPath)
				assert.Equal(t, tc.expected, result, "Request: %s, Policy: %s", tc.requestPath, tc.policyPath)
			})
		}
	})

	t.Run("Resource Type Classification", func(t *testing.T) {

		publicResources := []string{"Questionnaire", "ResearchStudy", "Organization", "PractitionerRole", "Slot"}
		for _, resource := range publicResources {
			t.Run("Public_"+resource, func(t *testing.T) {
				assert.True(t, utils.IsPublicResource(resource), "%s should be classified as public", resource)
				assert.False(t, utils.RequiresPatientOwnership(resource), "%s should not require patient ownership", resource)
			})
		}

		patientResources := []string{"Patient", "Appointment", "Observation", "QuestionnaireResponse", "Encounter"}
		for _, resource := range patientResources {
			t.Run("PatientSpecific_"+resource, func(t *testing.T) {
				assert.False(t, utils.IsPublicResource(resource), "%s should not be classified as public", resource)
				assert.True(t, utils.RequiresPatientOwnership(resource), "%s should require patient ownership", resource)
			})
		}

		testCases := []struct {
			name     string
			path     string
			expected string
		}{
			{
				name:     "FHIR path",
				path:     "/fhir/Patient/123",
				expected: "Patient",
			},
			{
				name:     "Direct path",
				path:     "/Patient/123",
				expected: "Patient",
			},
			{
				name:     "FHIR path with query",
				path:     "/fhir/Organization?_elements=name,address",
				expected: "Organization",
			},
			{
				name:     "Complex path",
				path:     "/fhir/Appointment?actor=Patient/123&slot.start=ge2025-01-01",
				expected: "Appointment",
			},
		}

		for _, tc := range testCases {
			t.Run("Extract_"+tc.name, func(t *testing.T) {
				result := utils.ExtractResourceTypeFromPath(tc.path)
				assert.Equal(t, tc.expected, result, "Path: %s", tc.path)
			})
		}
	})
}
