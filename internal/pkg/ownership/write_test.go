package ownership

import (
	"testing"

	"konsulin-service/internal/pkg/constvars"

	"github.com/stretchr/testify/assert"
)

func TestValidateWriteBody_PatientCreatesOwnCondition(t *testing.T) {
	oc := patientOC("pat-1")
	body := `{"resourceType":"Condition","subject":{"reference":"Patient/pat-1"}}`
	assert.NoError(t, ValidateWriteBody([]byte(body), constvars.ResourceCondition, oc, false))

	mismatch := `{"resourceType":"Condition","subject":{"reference":"Patient/pat-2"}}`
	assert.Error(t, ValidateWriteBody([]byte(mismatch), constvars.ResourceCondition, oc, false))

	// lenient POST: missing subject is allowed
	missing := `{"resourceType":"Condition","code":{"text":"x"}}`
	assert.NoError(t, ValidateWriteBody([]byte(missing), constvars.ResourceCondition, oc, false))

	// strict PUT: missing subject is denied
	assert.Error(t, ValidateWriteBody([]byte(missing), constvars.ResourceCondition, oc, true))
}

func TestValidateWriteBody_PatientConsentOwnPatient(t *testing.T) {
	oc := patientOC("pat-1")
	assert.NoError(t, ValidateWriteBody([]byte(`{"resourceType":"Consent","patient":{"reference":"Patient/pat-1"}}`), constvars.ResourceConsent, oc, false))
	assert.Error(t, ValidateWriteBody([]byte(`{"resourceType":"Consent","patient":{"reference":"Patient/pat-2"}}`), constvars.ResourceConsent, oc, false))
}

func TestValidateWriteBody_CommunicationSenderOwnPatient(t *testing.T) {
	oc := patientOC("pat-1")
	assert.NoError(t, ValidateWriteBody([]byte(`{"resourceType":"Communication","sender":{"reference":"Patient/pat-1"}}`), constvars.ResourceCommunication, oc, false))
	assert.Error(t, ValidateWriteBody([]byte(`{"resourceType":"Communication","sender":{"reference":"Patient/pat-2"}}`), constvars.ResourceCommunication, oc, false))
	// strict PUT: sender required
	assert.Error(t, ValidateWriteBody([]byte(`{"resourceType":"Communication"}`), constvars.ResourceCommunication, oc, true))
}

func TestValidateWriteBody_PatientAppointmentParticipantActor(t *testing.T) {
	oc := patientOC("pat-1")
	body := `{"resourceType":"Appointment","participant":[{"actor":{"reference":"Patient/pat-1"}}]}`
	assert.NoError(t, ValidateWriteBody([]byte(body), constvars.ResourceAppointment, oc, false))
	mismatch := `{"resourceType":"Appointment","participant":[{"actor":{"reference":"Patient/pat-2"}}]}`
	assert.Error(t, ValidateWriteBody([]byte(mismatch), constvars.ResourceAppointment, oc, false))
}

func TestValidateWriteBody_PatientResearchSubjectIndividual(t *testing.T) {
	oc := patientOC("pat-1")
	assert.NoError(t, ValidateWriteBody([]byte(`{"resourceType":"ResearchSubject","individual":{"reference":"Patient/pat-1"}}`), constvars.ResourceResearchSubject, oc, false))
	assert.Error(t, ValidateWriteBody([]byte(`{"resourceType":"ResearchSubject","individual":{"reference":"Patient/pat-2"}}`), constvars.ResourceResearchSubject, oc, false))
}

func TestValidateWriteBody_PatientOwnID(t *testing.T) {
	oc := patientOC("pat-1")
	assert.NoError(t, ValidateWriteBody([]byte(`{"resourceType":"Patient","id":"pat-1"}`), constvars.ResourcePatient, oc, false))
	assert.Error(t, ValidateWriteBody([]byte(`{"resourceType":"Patient","id":"pat-2"}`), constvars.ResourcePatient, oc, false))
}

func TestValidateWriteBody_PractitionerOwnID(t *testing.T) {
	oc := practitionerOC("prac-1")
	assert.NoError(t, ValidateWriteBody([]byte(`{"resourceType":"Practitioner","id":"prac-1"}`), constvars.ResourcePractitioner, oc, false))
	assert.Error(t, ValidateWriteBody([]byte(`{"resourceType":"Practitioner","id":"prac-2"}`), constvars.ResourcePractitioner, oc, false))
}

func TestValidateWriteBody_PractitionerRoleRequiresOwnPractitionerRef(t *testing.T) {
	oc := practitionerOC("prac-1")
	// plain practitioner can only PUT a role bound to themselves
	body := `{"resourceType":"PractitionerRole","practitioner":{"reference":"Practitioner/prac-1"}}`
	assert.NoError(t, ValidateWriteBody([]byte(body), constvars.ResourcePractitionerRole, oc, true))
	mismatch := `{"resourceType":"PractitionerRole","practitioner":{"reference":"Practitioner/prac-2"}}`
	assert.Error(t, ValidateWriteBody([]byte(mismatch), constvars.ResourcePractitionerRole, oc, true))
}

func TestValidateWriteBody_ClinicAdminBypassesPractitionerRoleCheck(t *testing.T) {
	admin := practitionerOC("prac-1", constvars.FhirPractitionerRoleSystemSnomed+"|"+constvars.FhirPractitionerRoleCodeAdministrativeStaff)
	// clinic admin manages roles for other practitioners
	body := `{"resourceType":"PractitionerRole","practitioner":{"reference":"Practitioner/prac-2"}}`
	assert.NoError(t, ValidateWriteBody([]byte(body), constvars.ResourcePractitionerRole, admin, true))
}

func TestValidateWriteBody_PublicResourceNoConstraints(t *testing.T) {
	oc := testOC()
	assert.NoError(t, ValidateWriteBody([]byte(`{"resourceType":"PlanDefinition","id":"pd-1"}`), "PlanDefinition", oc, false))
	assert.NoError(t, ValidateWriteBody([]byte(`{"resourceType":"Location","id":"loc-1"}`), constvars.ResourceLocation, oc, false))
}

func TestValidateWriteBody_UnclassifiedTypeDenied(t *testing.T) {
	oc := patientOC("pat-1")
	assert.Error(t, ValidateWriteBody([]byte(`{"resourceType":"AdverseEvent"}`), "AdverseEvent", oc, false))
}
