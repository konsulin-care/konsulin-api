package roles

import (
	"testing"

	"konsulin-service/internal/pkg/ownership"
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

	// The Casbin enforcer grants path-level access by role/method/resource path.
	// ID-level ownership is enforced downstream in ownsResource (middlewares/auth.go)
	// for writes and post-request filtering (middlewares/proxy.go) for reads, which
	// this enforcer-only test does not wire up.
	t.Run("Patient Role Path-Level Access", func(t *testing.T) {

		allowed, err := enforcer.Enforce("Patient", "GET", "/fhir/Patient/patient-123")
		assert.NoError(t, err)
		assert.True(t, allowed, "Patient policy should grant access to their own patient resource path")

		allowed, err = enforcer.Enforce("Patient", "GET", "/fhir/Patient/other-patient-456")
		assert.NoError(t, err)
		assert.True(t, allowed, "policy grants path-level access; ID-level ownership is enforced downstream")

		allowed, err = enforcer.Enforce("Patient", "GET", "/fhir/Appointment?actor=Patient/patient-123")
		assert.NoError(t, err)
		assert.True(t, allowed, "Patient policy should grant access to appointment paths")

		allowed, err = enforcer.Enforce("Patient", "GET", "/fhir/Appointment?actor=Patient/other-patient-456")
		assert.NoError(t, err)
		assert.True(t, allowed, "policy grants path-level access; ID-level ownership is enforced downstream")

		allowed, err = enforcer.Enforce("Patient", "GET", "/fhir/Observation?subject=Patient/patient-123")
		assert.NoError(t, err)
		assert.True(t, allowed, "Patient policy should grant access to observation paths")

		allowed, err = enforcer.Enforce("Patient", "GET", "/fhir/Observation?subject=Patient/other-patient-456")
		assert.NoError(t, err)
		assert.True(t, allowed, "policy grants path-level access; ID-level ownership is enforced downstream")
	})

	t.Run("Patient Role Complex Appointment Queries", func(t *testing.T) {

		allowed, err := enforcer.Enforce("Patient", "GET", "/fhir/Appointment?actor=Patient/patient-123&slot.start=ge2025-01-01T00:00:00+00:00&_include=Appointment:actor:PractitionerRole&_include:iterate=PractitionerRole:practitioner&_include=Appointment:slot")
		assert.NoError(t, err)
		assert.True(t, allowed, "Patient should be able to access their own appointments with complex query")

		allowed, err = enforcer.Enforce("Patient", "GET", "/fhir/Appointment?actor=Patient/other-patient-456&slot.start=ge2025-01-01T00:00:00+00:00&_include=Appointment:actor:PractitionerRole&_include:iterate=PractitionerRole:practitioner&_include=Appointment:slot")
		assert.NoError(t, err)
		assert.True(t, allowed, "policy grants path-level access regardless of actor; ID-level ownership is enforced downstream")
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
			{
				name:        "Resource ID sub-path match",
				requestPath: "/fhir/PractitionerRole/DGKAVBCXVMWT6UOE",
				policyPath:  "/fhir/PractitionerRole",
				expected:    true,
			},
			{
				name:        "Resource ID sub-path match with query",
				requestPath: "/fhir/PractitionerRole/DGKAVBCXVMWT6UOE?_elements=id,active",
				policyPath:  "/fhir/PractitionerRole",
				expected:    true,
			},
			{
				name:        "Patient resource ID sub-path match",
				requestPath: "/fhir/Patient/ABC123",
				policyPath:  "/fhir/Patient",
				expected:    true,
			},
			{
				name:        "False positive prevention - similar prefix",
				requestPath: "/fhir/PractitionerRoleABC",
				policyPath:  "/fhir/PractitionerRole",
				expected:    false,
			},
			{
				name:        "Policy path with trailing slash - exact match required",
				requestPath: "/fhir/Organization/",
				policyPath:  "/fhir/Organization/",
				expected:    true,
			},
			{
				name:        "Policy path with trailing slash - sub-path not allowed",
				requestPath: "/fhir/Organization/123",
				policyPath:  "/fhir/Organization/",
				expected:    false,
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

		// Ownership classification lives in the declarative spec
		// (internal/pkg/ownership); the RBAC policy gates paths separately.
		publicResources := []string{"Questionnaire", "ResearchStudy", "Organization", "PractitionerRole", "Slot", "Schedule"}
		for _, resource := range publicResources {
			t.Run("Public_"+resource, func(t *testing.T) {
				rule, ok := ownership.Rule(resource)
				assert.True(t, ok && rule.Scope == ownership.ScopePublic, "%s should be classified as public", resource)
			})
		}

		patientOwned := []string{"Patient", "Appointment", "Observation", "Encounter", "Consent", "ResearchSubject", "Communication"}
		for _, resource := range patientOwned {
			t.Run("PatientOwned_"+resource, func(t *testing.T) {
				rule, ok := ownership.Rule(resource)
				assert.True(t, ok, "%s should have an ownership rule", resource)
				assert.True(t, ruleHasRefTarget(rule, "Patient"), "%s should be patient-owned", resource)
			})
		}

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
			{
				name:        "Resource ID sub-path match",
				requestPath: "/fhir/PractitionerRole/DGKAVBCXVMWT6UOE",
				policyPath:  "/fhir/PractitionerRole",
				expected:    true,
			},
			{
				name:        "Resource ID sub-path match with query",
				requestPath: "/fhir/PractitionerRole/DGKAVBCXVMWT6UOE?_elements=id,active",
				policyPath:  "/fhir/PractitionerRole",
				expected:    true,
			},
			{
				name:        "Patient resource ID sub-path match",
				requestPath: "/fhir/Patient/ABC123",
				policyPath:  "/fhir/Patient",
				expected:    true,
			},
			{
				name:        "False positive prevention - similar prefix",
				requestPath: "/fhir/PractitionerRoleABC",
				policyPath:  "/fhir/PractitionerRole",
				expected:    false,
			},
			{
				name:        "Policy path with trailing slash - exact match required",
				requestPath: "/fhir/Organization/",
				policyPath:  "/fhir/Organization/",
				expected:    true,
			},
			{
				name:        "Policy path with trailing slash - sub-path not allowed",
				requestPath: "/fhir/Organization/123",
				policyPath:  "/fhir/Organization/",
				expected:    false,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				result := utils.PathMatch(tc.requestPath, tc.policyPath)
				assert.Equal(t, tc.expected, result, "Request: %s, Policy: %s", tc.requestPath, tc.policyPath)
			})
		}
	})

	t.Run("Consent and ResearchSubject Policy", func(t *testing.T) {
		for _, res := range []string{"Consent", "ResearchSubject"} {
			// Patient: GET/POST/PUT allowed.
			for _, method := range []string{"GET", "POST", "PUT"} {
				allowed, err := enforcer.Enforce("Patient", method, "/fhir/"+res)
				assert.NoError(t, err)
				assert.True(t, allowed, "Patient %s /fhir/%s should be allowed", method, res)
			}

			// Researcher: GET allowed, writes denied.
			allowed, err := enforcer.Enforce("Researcher", "GET", "/fhir/"+res)
			assert.NoError(t, err)
			assert.True(t, allowed, "Researcher GET /fhir/%s should be allowed", res)
			for _, method := range []string{"POST", "PUT", "DELETE"} {
				allowed, err = enforcer.Enforce("Researcher", method, "/fhir/"+res)
				assert.NoError(t, err)
				assert.False(t, allowed, "Researcher %s /fhir/%s should be denied", method, res)
			}

			// Superadmin: GET allowed.
			allowed, err = enforcer.Enforce("Superadmin", "GET", "/fhir/"+res)
			assert.NoError(t, err)
			assert.True(t, allowed, "Superadmin GET /fhir/%s should be allowed", res)

			// Guest / Practitioner / Clinic Admin: denied.
			for _, role := range []string{"Guest", "Practitioner", "Clinic Admin"} {
				for _, method := range []string{"GET", "POST", "PUT"} {
					allowed, err = enforcer.Enforce(role, method, "/fhir/"+res)
					assert.NoError(t, err)
					assert.False(t, allowed, "%s %s /fhir/%s should be denied", role, method, res)
				}
			}
		}

		// Prefix matching: /fhir/Consent covers sub-paths and query params.
		allowed, err := enforcer.Enforce("Patient", "GET", "/fhir/Consent/consent-123")
		assert.NoError(t, err)
		assert.True(t, allowed, "Patient GET /fhir/Consent/{id} should be allowed via prefix match")

		allowed, err = enforcer.Enforce("Patient", "GET", "/fhir/ResearchSubject/rs-1?patient=Patient/pat-1")
		assert.NoError(t, err)
		assert.True(t, allowed, "Patient GET /fhir/ResearchSubject/{id} with query should be allowed via prefix match")
	})
}
