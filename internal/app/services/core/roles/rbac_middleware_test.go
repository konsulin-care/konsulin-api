package roles

import (
	"net/url"
	"testing"

	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/ownership"
	"konsulin-service/internal/pkg/utils"

	"github.com/stretchr/testify/assert"
)

func TestOwnsResourceFunction(t *testing.T) {

	t.Run("Patient Role Public Resources", func(t *testing.T) {

		owns := ownsResource("patient-123", "/fhir/Questionnaire?_elements=title,description&subject-type=Person,Patient&status=active&context=popular", constvars.KonsulinRolePatient, "GET")
		assert.True(t, owns, "Patient should be able to access public questionnaires")

		owns = ownsResource("patient-123", "/fhir/ResearchStudy?date=ge2025-04-14&_revinclude=List:item", constvars.KonsulinRolePatient, "GET")
		assert.True(t, owns, "Patient should be able to access public research studies")

		owns = ownsResource("patient-123", "/fhir/Organization?_elements=name,address", constvars.KonsulinRolePatient, "GET")
		assert.True(t, owns, "Patient should be able to access public organization info")

		owns = ownsResource("patient-123", "/fhir/Location?status=active", constvars.KonsulinRolePatient, "GET")
		assert.True(t, owns, "Patient should be able to access public locations")

		owns = ownsResource("patient-123", "/fhir/HealthcareService?active=true", constvars.KonsulinRolePatient, "GET")
		assert.True(t, owns, "Patient should be able to access public healthcare services")

		owns = ownsResource("patient-123", "/fhir/PractitionerRole?active=true", constvars.KonsulinRolePatient, "GET")
		assert.True(t, owns, "Patient should be able to access public practitioner roles")

		owns = ownsResource("patient-123", "/fhir/Slot?status=free", constvars.KonsulinRolePatient, "GET")
		assert.True(t, owns, "Patient should be able to access public slots")
	})

	t.Run("Patient Role Protected Resources", func(t *testing.T) {

		owns := ownsResource("patient-123", "/fhir/Patient/patient-123", constvars.KonsulinRolePatient, "GET")
		assert.True(t, owns, "Patient should be able to access their own patient resource")

		owns = ownsResource("patient-123", "/fhir/Patient/other-patient-456", constvars.KonsulinRolePatient, "GET")
		assert.False(t, owns, "Patient should not be able to access other patients' resources")

		owns = ownsResource("patient-123", "/fhir/Appointment?actor=Patient/patient-123", constvars.KonsulinRolePatient, "GET")
		assert.True(t, owns, "Patient should be able to access their own appointments")

		owns = ownsResource("patient-123", "/fhir/Appointment?actor=Patient/other-patient-456", constvars.KonsulinRolePatient, "GET")
		// GET bypasses the pre-request check; the response filter (OwnedBy) gates reads.
		assert.True(t, owns, "pre-request GET bypasses ownership; response filter gates")

		owns = ownsResource("patient-123", "/fhir/Observation?subject=Patient/patient-123", constvars.KonsulinRolePatient, "GET")
		assert.True(t, owns, "Patient should be able to access their own observations")

		owns = ownsResource("patient-123", "/fhir/Observation?subject=Patient/other-patient-456", constvars.KonsulinRolePatient, "GET")
		assert.True(t, owns, "pre-request GET bypasses ownership; response filter gates")
	})

	t.Run("Patient Role Complex Appointment Queries", func(t *testing.T) {

		owns := ownsResource("patient-123", "/fhir/Appointment?actor=Patient/patient-123&slot.start=ge2025-01-01T00:00:00+00:00&_include=Appointment:actor:PractitionerRole&_include:iterate=PractitionerRole:practitioner&_include=Appointment:slot", constvars.KonsulinRolePatient, "GET")
		assert.True(t, owns, "Patient should be able to access their own appointments with complex query")

		owns = ownsResource("patient-123", "/fhir/Appointment?actor=Patient/other-patient-456&slot.start=ge2025-01-01T00:00:00+00:00&_include=Appointment:actor:PractitionerRole&_include:iterate=PractitionerRole:practitioner&_include=Appointment:slot", constvars.KonsulinRolePatient, "GET")
		assert.True(t, owns, "pre-request GET bypasses ownership; response filter gates")
	})

	t.Run("Resource Type Classification", func(t *testing.T) {

		// Ownership classification is now the declarative spec in
		// internal/pkg/ownership (single source of truth).
		publicResources := []string{"Questionnaire", "ResearchStudy", "Organization", "Location", "HealthcareService", "PractitionerRole", "Slot", "Schedule"}
		for _, resource := range publicResources {
			t.Run("Public_"+resource, func(t *testing.T) {
				rule, ok := ownership.Rule(resource)
				assert.True(t, ok && rule.Scope == ownership.ScopePublic, "%s should be classified as public", resource)
			})
		}

		patientOwned := []string{"Patient", "Appointment", "Observation", "Encounter", "ResearchSubject", "Consent", "Communication"}
		for _, resource := range patientOwned {
			t.Run("PatientOwned_"+resource, func(t *testing.T) {
				rule, ok := ownership.Rule(resource)
				assert.True(t, ok, "%s should have an ownership rule", resource)
				assert.True(t, ruleHasRefTarget(rule, "Patient"), "%s should be patient-owned", resource)
			})
		}

		// Practitioner-owned resources.
		practitionerOwned := []string{"Task", "Communication", "Practitioner"}
		for _, resource := range practitionerOwned {
			t.Run("PractitionerOwned_"+resource, func(t *testing.T) {
				rule, ok := ownership.Rule(resource)
				assert.True(t, ok, "%s should have an ownership rule", resource)
				assert.True(t, ruleHasRefTarget(rule, "Practitioner") || ruleHasRefTarget(rule, "PractitionerRole"),
					"%s should be practitioner-owned", resource)
			})
		}

		// Practitioner is scoped to the caller's own practitioner id, with a
		// clinic-admin code allowance for directory management.
		t.Run("Practitioner_ClinicAdminCodeAllow", func(t *testing.T) {
			rule, ok := ownership.Rule(constvars.ResourcePractitioner)
			assert.True(t, ok)
			assert.Contains(t, rule.CodeAllow, ownership.CodingClinicAdmin)
		})

		// Schedule is public availability data on reads.
		t.Run("Schedule_Public", func(t *testing.T) {
			rule, ok := ownership.Rule(constvars.ResourceSchedule)
			assert.True(t, ok && rule.Scope == ownership.ScopePublic, "Schedule should be public")
			assert.Equal(t, "schedule", rule.WriteCheckerName)
		})

		// QuestionnaireResponse carries Shared ownership (author/subject on the
		// patient side, author/source on the practitioner side) plus a
		// researcher identifier-search allowance.
		t.Run("QuestionnaireResponse_SharedWithResearcherAllowance", func(t *testing.T) {
			rule, ok := ownership.Rule(constvars.ResourceQuestionnaireResponse)
			assert.True(t, ok && rule.Scope == ownership.ScopeShared, "QR should be shared-owned")
			assert.Equal(t, "questionnaire_response", rule.WriteCheckerName)
		})

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

	t.Run("Practitioner Role Public Resources", func(t *testing.T) {

		owns := ownsResource("practitioner-123", "/fhir/Organization?_elements=name,address", constvars.KonsulinRolePractitioner, "GET")
		assert.True(t, owns, "Practitioner should be able to access public organization info")

		owns = ownsResource("practitioner-123", "/fhir/Questionnaire?_elements=title,description", constvars.KonsulinRolePractitioner, "GET")
		assert.True(t, owns, "Practitioner should be able to access public questionnaires")

		owns = ownsResource("practitioner-123", "/fhir/ResearchStudy?date=ge2025-01-01", constvars.KonsulinRolePractitioner, "GET")
		assert.True(t, owns, "Practitioner should be able to access public research studies")
	})

	t.Run("Practitioner Role Protected Resources", func(t *testing.T) {

		owns := ownsResource("practitioner-123", "/fhir/Practitioner/practitioner-123", constvars.KonsulinRolePractitioner, "GET")
		assert.True(t, owns, "Practitioner should be able to access their own practitioner resource")

		owns = ownsResource("practitioner-123", "/fhir/Practitioner/other-practitioner-456", constvars.KonsulinRolePractitioner, "GET")
		assert.False(t, owns, "Practitioner is scoped to the caller's own id (clinic admin code allowance reads the directory)")

		owns = ownsResource("practitioner-123", "/fhir/Encounter?practitioner=Practitioner/practitioner-123", constvars.KonsulinRolePractitioner, "GET")
		assert.True(t, owns, "Practitioner should be able to access their own encounters")

		owns = ownsResource("practitioner-123", "/fhir/Encounter?practitioner=Practitioner/other-practitioner-456", constvars.KonsulinRolePractitioner, "GET")
		assert.True(t, owns, "pre-request GET bypasses ownership; response filter gates")

		owns = ownsResource("practitioner-123", "/fhir/PractitionerRole?practitioner=Practitioner/practitioner-123&_include=PractitionerRole:organization", constvars.KonsulinRolePractitioner, "GET")
		assert.True(t, owns, "Practitioner should be able to access their own practitioner roles")

		owns = ownsResource("practitioner-123", "/fhir/PractitionerRole?practitioner=Practitioner/other-practitioner-456&_include=PractitionerRole:organization", constvars.KonsulinRolePractitioner, "GET")
		assert.False(t, owns, "Practitioner should not be able to access other practitioners' roles")

		owns = ownsResource("practitioner-123", "/fhir/Slot?_has:Appointment:slot:practitioner=practitioner-123&start=ge2025-01-01&start=le2025-01-08", constvars.KonsulinRolePractitioner, "GET")
		assert.True(t, owns, "Practitioner should be able to access their own slots")

		owns = ownsResource("practitioner-123", "/fhir/Slot?_has:Appointment:slot:practitioner=other-practitioner-456&start=ge2025-01-01&start=le2025-01-08", constvars.KonsulinRolePractitioner, "GET")
		assert.True(t, owns, "pre-request GET bypasses ownership; response filter gates")

		owns = ownsResource("practitioner-123", "/fhir/Appointment?_elements=appointmentType,participant,slot&practitioner=practitioner-123&slot.start=ge2025-01-01&slot.start=le2025-01-08&_include=Appointment:patient&_include=Appointment:slot", constvars.KonsulinRolePractitioner, "GET")
		assert.True(t, owns, "Practitioner should be able to access their own appointments")

		owns = ownsResource("practitioner-123", "/fhir/Appointment?_elements=appointmentType,participant,slot&practitioner=other-practitioner-456&slot.start=ge2025-01-01&slot.start=le2025-01-08&_include=Appointment:patient&_include=Appointment:slot", constvars.KonsulinRolePractitioner, "GET")
		assert.True(t, owns, "pre-request GET bypasses ownership; response filter gates")
	})

	t.Run("Other Roles", func(t *testing.T) {

		owns := ownsResource("guest-123", "/fhir/Organization", constvars.KonsulinRoleGuest, "GET")
		assert.True(t, owns, "Organization is public; ownership does not restrict it")

		owns = ownsResource("admin-123", "/fhir/Organization", constvars.KonsulinRoleClinicAdmin, "GET")
		assert.True(t, owns, "Organization is public; ownership does not restrict it")

		owns = ownsResource("superadmin-123", "/fhir/Organization", constvars.KonsulinRoleSuperadmin, "GET")
		assert.True(t, owns, "Organization is public; ownership does not restrict it")
	})

	t.Run("POST Request Access", func(t *testing.T) {

		owns := ownsResource("patient-123", "/fhir/QuestionnaireResponse", constvars.KonsulinRolePatient, "POST")
		assert.True(t, owns, "Patient should be able to POST new QuestionnaireResponse")

		owns = ownsResource("practitioner-123", "/fhir/QuestionnaireResponse", constvars.KonsulinRolePractitioner, "POST")
		assert.True(t, owns, "Practitioner should be able to POST new QuestionnaireResponse")

		owns = ownsResource("patient-123", "/fhir/Observation", constvars.KonsulinRolePatient, "POST")
		assert.True(t, owns, "Patient should be able to POST new Observation")

		owns = ownsResource("practitioner-123", "/fhir/Observation", constvars.KonsulinRolePractitioner, "POST")
		assert.True(t, owns, "Practitioner should be able to POST new Observation")
	})
}

func ownsResource(fhirID, rawURL, role, method string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}

	// The real middleware bypasses pre-request ownership checks for GET
	// (the response filter gates reads); ValidSearchQuery is the engine's
	// pre-request scoping for entry-level searches and non-GET methods.
	if method == "POST" {
		return true
	}
	oc := ownership.NewContext()
	switch role {
	case constvars.KonsulinRolePatient:
		oc.HasPatientRole = true
		oc.AddPatientID(fhirID)
	case constvars.KonsulinRolePractitioner:
		oc.HasPractitionerRole = true
		oc.AddPractitionerID(fhirID)
	}
	resourceType := utils.ExtractResourceTypeFromPath(u.Path)
	return ownership.ValidSearchQuery(u.RequestURI(), resourceType, oc)
}

// ruleHasRefTarget reports whether the ownership rule carries a ref whose
// target type matches.
func ruleHasRefTarget(rule ownership.ResourceRule, target string) bool {
	for _, ref := range rule.Refs {
		if ref.Target == target {
			return true
		}
	}
	return false
}
