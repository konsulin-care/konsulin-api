package roles

import (
	"testing"

	"konsulin-service/internal/pkg/ownership"
	"konsulin-service/internal/pkg/utils"

	"github.com/casbin/casbin/v2"
	"github.com/stretchr/testify/assert"
)

// newRBACEnforcer builds the Casbin enforcer from the RBAC model and policy
// files, registering the pathMatch function the policy references. Skips the
// test when the RBAC files are missing (e.g. package-only CI runs).
func newRBACEnforcer(t *testing.T) *casbin.Enforcer {
	t.Helper()
	enforcer, err := casbin.NewEnforcer("../../../../../resources/rbac_model.conf", "../../../../../resources/rbac_policy.csv")
	if err != nil {
		t.Skipf("Skipping test due to missing RBAC files: %v", err)
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
	return enforcer
}

// assertEnforce asserts that the enforcer grants the given role/method/path.
func assertEnforce(t *testing.T, e *casbin.Enforcer, role, method, path string, want bool) {
	t.Helper()
	allowed, err := e.Enforce(role, method, path)
	assert.NoError(t, err)
	assert.Equal(t, want, allowed, "%s %s %s", role, method, path)
}

func TestRBACPathLevelAccess(t *testing.T) {
	e := newRBACEnforcer(t)

	// Guest: public metadata and slot listings.
	for _, path := range []string{
		"/fhir/Organization?_elements=name,address",
		"/fhir/Slot?schedule.actor=PractitionerRole/123&start=2025-01-01",
		"/fhir/Slot?schedule.actor=PractitionerRole/456",
	} {
		assertEnforce(t, e, "Guest", "GET", path, true)
	}

	// Clinic Admin and Superadmin: same public paths.
	for _, path := range []string{
		"/fhir/Organization?_elements=name,address",
		"/fhir/Slot?schedule.actor=PractitionerRole/789&start=2025-01-01",
	} {
		assertEnforce(t, e, "Clinic Admin", "GET", path, true)
	}
	for _, path := range []string{
		"/fhir/Organization?_elements=name,address",
		"/fhir/Slot?schedule.actor=PractitionerRole/999&start=2025-01-01",
	} {
		assertEnforce(t, e, "Superadmin", "GET", path, true)
	}

	// Patient public resources.
	for _, path := range []string{
		"/fhir/Questionnaire?_elements=title,description&subject-type=Person,Patient&status=active&context=popular",
		"/fhir/ResearchStudy?date=ge2025-04-14&_revinclude=List:item",
		"/fhir/Organization?_elements=name,address",
		"/fhir/PractitionerRole?active=true",
		"/fhir/Slot?status=free",
	} {
		assertEnforce(t, e, "Patient", "GET", path, true)
	}

	// The Casbin enforcer grants path-level access by role/method/resource
	// path. ID-level ownership is enforced downstream in ownsResource
	// (middlewares/auth.go) for writes and post-request filtering
	// (middlewares/proxy.go) for reads, which this enforcer-only test does not
	// wire up.
	for _, path := range []string{
		"/fhir/Patient/patient-123",
		"/fhir/Patient/other-patient-456",
		"/fhir/Appointment?actor=Patient/patient-123",
		"/fhir/Appointment?actor=Patient/other-patient-456",
		"/fhir/Observation?subject=Patient/patient-123",
		"/fhir/Observation?subject=Patient/other-patient-456",
	} {
		assertEnforce(t, e, "Patient", "GET", path, true)
	}

	// Complex appointment queries are path-granted regardless of the actor.
	for _, path := range []string{
		"/fhir/Appointment?actor=Patient/patient-123&slot.start=ge2025-01-01T00:00:00+00:00&_include=Appointment:actor:PractitionerRole&_include:iterate=PractitionerRole:practitioner&_include=Appointment:slot",
		"/fhir/Appointment?actor=Patient/other-patient-456&slot.start=ge2025-01-01T00:00:00+00:00&_include=Appointment:actor:PractitionerRole&_include:iterate=PractitionerRole:practitioner&_include=Appointment:slot",
	} {
		assertEnforce(t, e, "Patient", "GET", path, true)
	}

	// Query parameter variations: base paths and query suffixes stay public.
	for _, path := range []string{
		"/fhir/Organization",
		"/fhir/Organization?_elements=name,address",
		"/fhir/Organization?_elements=title,description",
		"/fhir/Organization?_elements=name,address&status=active&_count=10",
		"/fhir/Slot?schedule.actor=PractitionerRole/123",
		"/fhir/Slot?schedule.actor=PractitionerRole/123&start=2025-01-01",
		"/fhir/Slot?schedule.actor=PractitionerRole/123&start=2025-01-01&status=free&_count=20",
	} {
		assertEnforce(t, e, "Guest", "GET", path, true)
	}
}

func TestPathMatchFunction(t *testing.T) {
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
}

func TestRBACResourceClassification(t *testing.T) {
	// Ownership classification lives in the declarative spec
	// (internal/pkg/ownership); the RBAC policy gates paths separately.
	for _, resource := range []string{"Questionnaire", "ResearchStudy", "Organization", "PractitionerRole", "Slot", "Schedule"} {
		t.Run("Public_"+resource, func(t *testing.T) {
			rule, ok := ownership.Rule(resource)
			assert.True(t, ok && rule.Scope == ownership.ScopePublic, "%s should be classified as public", resource)
		})
	}

	for _, resource := range []string{"Patient", "Appointment", "Observation", "Encounter", "Consent", "ResearchSubject", "Communication"} {
		t.Run("PatientOwned_"+resource, func(t *testing.T) {
			rule, ok := ownership.Rule(resource)
			assert.True(t, ok, "%s should have an ownership rule", resource)
			assert.True(t, ruleHasRefTarget(rule, "Patient"), "%s should be patient-owned", resource)
		})
	}
}

func TestRBACConsentResearchSubjectPolicy(t *testing.T) {
	e := newRBACEnforcer(t)
	for _, res := range []string{"Consent", "ResearchSubject"} {
		// Patient: GET/POST/PUT allowed.
		for _, method := range []string{"GET", "POST", "PUT"} {
			assertEnforce(t, e, "Patient", method, "/fhir/"+res, true)
		}

		// Researcher: GET allowed, writes denied.
		assertEnforce(t, e, "Researcher", "GET", "/fhir/"+res, true)
		for _, method := range []string{"POST", "PUT", "DELETE"} {
			assertEnforce(t, e, "Researcher", method, "/fhir/"+res, false)
		}

		// Superadmin: GET allowed.
		assertEnforce(t, e, "Superadmin", "GET", "/fhir/"+res, true)

		// Guest / Practitioner / Clinic Admin: denied.
		for _, role := range []string{"Guest", "Practitioner", "Clinic Admin"} {
			for _, method := range []string{"GET", "POST", "PUT"} {
				assertEnforce(t, e, role, method, "/fhir/"+res, false)
			}
		}
	}

	// Prefix matching: /fhir/Consent covers sub-paths and query params.
	assertEnforce(t, e, "Patient", "GET", "/fhir/Consent/consent-123", true)
	assertEnforce(t, e, "Patient", "GET", "/fhir/ResearchSubject/rs-1?patient=Patient/pat-1", true)
}
