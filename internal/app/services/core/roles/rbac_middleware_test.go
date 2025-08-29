package roles

import (
	"net/url"
	"strings"
	"testing"

	"konsulin-service/internal/pkg/constvars"
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
		assert.False(t, owns, "Patient should not be able to access other patients' appointments")

		owns = ownsResource("patient-123", "/fhir/Observation?subject=Patient/patient-123", constvars.KonsulinRolePatient, "GET")
		assert.True(t, owns, "Patient should be able to access their own observations")

		owns = ownsResource("patient-123", "/fhir/Observation?subject=Patient/other-patient-456", constvars.KonsulinRolePatient, "GET")
		assert.False(t, owns, "Patient should not be able to access other patients' observations")
	})

	t.Run("Patient Role Complex Appointment Queries", func(t *testing.T) {

		owns := ownsResource("patient-123", "/fhir/Appointment?actor=Patient/patient-123&slot.start=ge2025-01-01T00:00:00+00:00&_include=Appointment:actor:PractitionerRole&_include:iterate=PractitionerRole:practitioner&_include=Appointment:slot", constvars.KonsulinRolePatient, "GET")
		assert.True(t, owns, "Patient should be able to access their own appointments with complex query")

		owns = ownsResource("patient-123", "/fhir/Appointment?actor=Patient/other-patient-456&slot.start=ge2025-01-01T00:00:00+00:00&_include=Appointment:actor:PractitionerRole&_include:iterate=PractitionerRole:practitioner&_include=Appointment:slot", constvars.KonsulinRolePatient, "GET")
		assert.False(t, owns, "Patient should not be able to access other patients' appointments with complex query")
	})

	t.Run("Resource Type Classification", func(t *testing.T) {

		publicResources := []string{"Questionnaire", "ResearchStudy", "Organization", "Location", "HealthcareService", "PractitionerRole", "Slot"}
		for _, resource := range publicResources {
			t.Run("Public_"+resource, func(t *testing.T) {
				assert.True(t, utils.IsPublicResource(resource), "%s should be classified as public", resource)
				assert.False(t, utils.RequiresPatientOwnership(resource), "%s should not require patient ownership", resource)
				assert.False(t, utils.RequiresPractitionerOwnership(resource), "%s should not require practitioner ownership", resource)
			})
		}

		patientResources := []string{"Patient", "Appointment", "Observation", "QuestionnaireResponse", "Encounter"}
		for _, resource := range patientResources {
			t.Run("PatientSpecific_"+resource, func(t *testing.T) {
				assert.False(t, utils.IsPublicResource(resource), "%s should not be classified as public", resource)
				assert.True(t, utils.RequiresPatientOwnership(resource), "%s should require patient ownership", resource)
			})
		}

		practitionerResources := []string{"Practitioner", "Schedule"}
		for _, resource := range practitionerResources {
			t.Run("PractitionerSpecific_"+resource, func(t *testing.T) {
				assert.False(t, utils.IsPublicResource(resource), "%s should not be classified as public", resource)
				assert.False(t, utils.RequiresPatientOwnership(resource), "%s should not require patient ownership", resource)
				assert.True(t, utils.RequiresPractitionerOwnership(resource), "%s should require practitioner ownership", resource)
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
		assert.False(t, owns, "Practitioner should not be able to access other practitioners' resources")

		owns = ownsResource("practitioner-123", "/fhir/PractitionerRole?practitioner=Practitioner/practitioner-123&_include=PractitionerRole:organization", constvars.KonsulinRolePractitioner, "GET")
		assert.True(t, owns, "Practitioner should be able to access their own practitioner roles")

		owns = ownsResource("practitioner-123", "/fhir/PractitionerRole?practitioner=Practitioner/other-practitioner-456&_include=PractitionerRole:organization", constvars.KonsulinRolePractitioner, "GET")
		assert.False(t, owns, "Practitioner should not be able to access other practitioners' roles")

		owns = ownsResource("practitioner-123", "/fhir/Slot?_has:Appointment:slot:practitioner=practitioner-123&start=ge2025-01-01&start=le2025-01-08", constvars.KonsulinRolePractitioner, "GET")
		assert.True(t, owns, "Practitioner should be able to access their own slots")

		owns = ownsResource("practitioner-123", "/fhir/Slot?_has:Appointment:slot:practitioner=other-practitioner-456&start=ge2025-01-01&start=le2025-01-08", constvars.KonsulinRolePractitioner, "GET")
		assert.False(t, owns, "Practitioner should not be able to access other practitioners' slots")

		owns = ownsResource("practitioner-123", "/fhir/Appointment?_elements=appointmentType,participant,slot&practitioner=practitioner-123&slot.start=ge2025-01-01&slot.start=le2025-01-08&_include=Appointment:patient&_include=Appointment:slot", constvars.KonsulinRolePractitioner, "GET")
		assert.True(t, owns, "Practitioner should be able to access their own appointments")

		owns = ownsResource("practitioner-123", "/fhir/Appointment?_elements=appointmentType,participant,slot&practitioner=other-practitioner-456&slot.start=ge2025-01-01&slot.start=le2025-01-08&_include=Appointment:patient&_include=Appointment:slot", constvars.KonsulinRolePractitioner, "GET")
		assert.False(t, owns, "Practitioner should not be able to access other practitioners' appointments")
	})

	t.Run("Other Roles", func(t *testing.T) {

		owns := ownsResource("guest-123", "/fhir/Organization", constvars.KonsulinRoleGuest, "GET")
		assert.False(t, owns, "Guest role should not have ownership restrictions")

		owns = ownsResource("admin-123", "/fhir/Organization", constvars.KonsulinRoleClinicAdmin, "GET")
		assert.False(t, owns, "Clinic Admin role should not have ownership restrictions")

		owns = ownsResource("superadmin-123", "/fhir/Organization", constvars.KonsulinRoleSuperadmin, "GET")
		assert.False(t, owns, "Superadmin role should not have ownership restrictions")
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

	resourceType := utils.ExtractResourceTypeFromPath(u.Path)

	if method == "POST" {
		return true
	}

	if role == constvars.KonsulinRolePatient {

		if utils.IsPublicResource(resourceType) {
			return true
		}

		if utils.RequiresPatientOwnership(resourceType) {

			parts := strings.Split(strings.TrimPrefix(u.Path, "/"), "/")
			if len(parts) >= 2 {
				var res, id string

				if strings.EqualFold(parts[0], "fhir") {
					if len(parts) >= 3 {
						res, id = parts[1], parts[2]
					}
				} else {
					res, id = parts[0], parts[1]
				}

				if res == "Patient" && id == fhirID {
					return true
				}
			}

			q := u.Query()

			if p := q.Get("patient"); p != "" {
				id := strings.TrimPrefix(p, "Patient/")
				return id == fhirID
			}

			if s := q.Get("subject"); s != "" {
				id := strings.TrimPrefix(s, "Patient/")
				return id == fhirID
			}

			if a := q.Get("actor"); a != "" {
				id := strings.TrimPrefix(a, "Patient/")
				return id == fhirID
			}

			if qr := q.Get("questionnaire"); qr != "" {
				return true
			}

			return false
		}

		return false
	}

	if role == constvars.KonsulinRolePractitioner {

		if utils.IsPublicResource(resourceType) {

			q := u.Query()
			hasOwnershipParams := false

			if q.Get("practitioner") != "" || q.Get("actor") != "" {
				hasOwnershipParams = true
			}

			for key := range q {
				if strings.HasPrefix(key, "_has") && strings.Contains(key, "practitioner") {
					hasOwnershipParams = true
					break
				}
			}

			if !hasOwnershipParams {
				return true
			}

			parts := strings.Split(strings.TrimPrefix(u.Path, "/"), "/")
			if len(parts) >= 2 {
				var res, id string
				if strings.EqualFold(parts[0], "fhir") {
					if len(parts) >= 3 {
						res, id = parts[1], parts[2]
					}
				} else {
					res, id = parts[0], parts[1]
				}

				if res == "Practitioner" && id == fhirID {
					return true
				}
			}

			if p := q.Get("practitioner"); p != "" {
				id := strings.TrimPrefix(p, "Practitioner/")
				return id == fhirID
			}

			if a := q.Get("actor"); a != "" {
				id := strings.TrimPrefix(a, "Practitioner/")
				return id == fhirID
			}

			for key, values := range q {
				if strings.HasPrefix(key, "_has") && strings.Contains(key, "practitioner") {
					for _, value := range values {
						if value == fhirID {
							return true
						}
					}
				}
			}

			return false
		}

		if utils.RequiresPractitionerOwnership(resourceType) {

			parts := strings.Split(strings.TrimPrefix(u.Path, "/"), "/")
			if len(parts) >= 2 {
				var res, id string
				if strings.EqualFold(parts[0], "fhir") {
					if len(parts) >= 3 {
						res, id = parts[1], parts[2]
					}
				} else {
					res, id = parts[0], parts[1]
				}

				if res == "Practitioner" && id == fhirID {
					return true
				}
			}

			q := u.Query()

			if p := q.Get("practitioner"); p != "" {
				id := strings.TrimPrefix(p, "Practitioner/")
				return id == fhirID
			}

			if a := q.Get("actor"); a != "" {
				id := strings.TrimPrefix(a, "Practitioner/")
				return id == fhirID
			}

			for key, values := range q {
				if strings.HasPrefix(key, "_has") && strings.Contains(key, "practitioner") {
					for _, value := range values {
						if value == fhirID {
							return true
						}
					}
				}
			}

			return false
		}

		if resourceType == "Appointment" {
			q := u.Query()

			if p := q.Get("practitioner"); p != "" {
				id := strings.TrimPrefix(p, "Practitioner/")
				return id == fhirID
			}

			if a := q.Get("actor"); a != "" {
				id := strings.TrimPrefix(a, "Practitioner/")
				return id == fhirID
			}

			return false
		}

		return false
	}

	return false
}
