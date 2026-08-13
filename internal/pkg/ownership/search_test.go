package ownership

import (
	"testing"

	"konsulin-service/internal/pkg/constvars"

	"github.com/stretchr/testify/assert"
)

func TestValidSearchQuery_SingleResourceReadExempt(t *testing.T) {
	oc := testOC()
	assert.True(t, ValidSearchQuery("/QuestionnaireResponse/qr-123", constvars.ResourceQuestionnaireResponse, oc))
}

func TestValidSearchQuery_CountSummaryPublic(t *testing.T) {
	for _, rc := range []string{constvars.ResourceQuestionnaireResponse, constvars.ResourceCommunication} {
		assert.True(t, ValidSearchQuery("/"+rc+"?_summary=count", rc, testOC()))
	}
}

func TestValidSearchQuery_PatientOwnQRByAuthorPatientSubject(t *testing.T) {
	oc := patientOC("pat-1")
	assert.True(t, ValidSearchQuery("/QuestionnaireResponse?author=Patient/pat-1", constvars.ResourceQuestionnaireResponse, oc))
	assert.True(t, ValidSearchQuery("/QuestionnaireResponse?patient=pat-1", constvars.ResourceQuestionnaireResponse, oc))
	assert.True(t, ValidSearchQuery("/QuestionnaireResponse?subject=Patient/pat-1", constvars.ResourceQuestionnaireResponse, oc))
	assert.False(t, ValidSearchQuery("/QuestionnaireResponse?author=Patient/pat-2", constvars.ResourceQuestionnaireResponse, oc))
	// bare search denied
	assert.False(t, ValidSearchQuery("/QuestionnaireResponse?questionnaire=https://konsulin.care/fhir/Questionnaire/phq2", constvars.ResourceQuestionnaireResponse, oc))
}

func TestValidSearchQuery_PatientOwnCommunicationBySenderOrRecipient(t *testing.T) {
	oc := patientOC("pat-1")
	assert.True(t, ValidSearchQuery("/Communication?sender=Patient/pat-1&topic=research-referral", constvars.ResourceCommunication, oc))
	assert.True(t, ValidSearchQuery("/Communication?recipient=Patient/pat-2,Patient/pat-1", constvars.ResourceCommunication, oc))
	assert.False(t, ValidSearchQuery("/Communication?sender=Patient/pat-2", constvars.ResourceCommunication, oc))
	assert.False(t, ValidSearchQuery("/Communication?_count=500", constvars.ResourceCommunication, oc))
}

func TestValidSearchQuery_GuestQRByIdentifier(t *testing.T) {
	oc := testOC()
	assert.True(t, ValidSearchQuery("/QuestionnaireResponse?identifier=https://login.konsulin.care/guestid%7Cguest-123", constvars.ResourceQuestionnaireResponse, oc))
	assert.False(t, ValidSearchQuery("/QuestionnaireResponse?questionnaire=https://konsulin.care/fhir/Questionnaire/phq2", constvars.ResourceQuestionnaireResponse, oc))
	assert.False(t, ValidSearchQuery("/Communication?sender=Patient/pat-1", constvars.ResourceCommunication, oc))
}

func TestValidSearchQuery_PractitionerExemptFromScopedEntry(t *testing.T) {
	oc := practitionerOC("prac-1")
	assert.True(t, ValidSearchQuery("/QuestionnaireResponse?patient=pat-2", constvars.ResourceQuestionnaireResponse, oc))
	assert.True(t, ValidSearchQuery("/Communication?subject=Patient/pat-2", constvars.ResourceCommunication, oc))
}

func TestValidSearchQuery_ResearcherScopedByTopicSenderRecipientSubject(t *testing.T) {
	researcher := practitionerOC("prac-1", constvars.FhirPractitionerRoleSystemHL7+"|"+constvars.FhirPractitionerRoleCodeResearcher)
	assert.True(t, ValidSearchQuery("/Communication?sender=Patient/pat-1&topic=research-referral", constvars.ResourceCommunication, researcher))
	assert.True(t, ValidSearchQuery("/Communication?topic=research-referral", constvars.ResourceCommunication, researcher))
	assert.True(t, ValidSearchQuery("/Communication?recipient=Patient/pat-9", constvars.ResourceCommunication, researcher))
	assert.True(t, ValidSearchQuery("/Communication?subject=Patient/pat-9", constvars.ResourceCommunication, researcher))
	assert.False(t, ValidSearchQuery("/Communication?_count=500", constvars.ResourceCommunication, researcher))

	// QR by identifier
	assert.True(t, ValidSearchQuery("/QuestionnaireResponse?identifier=https://login.konsulin.care/guestid%7Cguest-123", constvars.ResourceQuestionnaireResponse, researcher))
	// bare QR search denied for researcher
	assert.False(t, ValidSearchQuery("/QuestionnaireResponse?questionnaire=https://konsulin.care/fhir/Questionnaire/phq2", constvars.ResourceQuestionnaireResponse, researcher))
}

func TestValidSearchQuery_PlainPractitionerExemptFromScopedEntry(t *testing.T) {
	// A plain practitioner (no researcher coding) keeps the legacy practitioner
	// pre-request exemption; the response filter (OwnedBy) is the gate that
	// denies them the researcher topic-based read allowance.
	plain := practitionerOC("prac-1")
	assert.True(t, ValidSearchQuery("/Communication?topic=research-referral", constvars.ResourceCommunication, plain))
	assert.True(t, ValidSearchQuery("/Communication?_count=500", constvars.ResourceCommunication, plain))
}

func TestValidSearchQuery_SuperadminCommunicationExempt(t *testing.T) {
	oc := testOC()
	oc.HasSuperadminRole = true
	assert.True(t, ValidSearchQuery("/Communication?_count=500", constvars.ResourceCommunication, oc))
}

func TestValidSearchQuery_PatientDeleteOwnPath(t *testing.T) {
	oc := patientOC("pat-1")
	assert.True(t, ValidSearchQuery("/Patient/pat-1", constvars.ResourcePatient, oc))
	assert.False(t, ValidSearchQuery("/Patient/pat-2", constvars.ResourcePatient, oc))
}

func TestValidSearchQuery_PractitionerDeleteOwnPath(t *testing.T) {
	oc := practitionerOC("prac-1")
	assert.True(t, ValidSearchQuery("/Practitioner/prac-1", constvars.ResourcePractitioner, oc))
	assert.False(t, ValidSearchQuery("/Practitioner/prac-2", constvars.ResourcePractitioner, oc))
}

func TestValidSearchQuery_PractitionerRolePublicScopedByPractitionerParam(t *testing.T) {
	oc := practitionerOC("prac-1")
	// no ownership param -> public listing allowed
	assert.True(t, ValidSearchQuery("/PractitionerRole?_count=50", constvars.ResourcePractitionerRole, oc))
	// scoped to own practitioner -> allowed
	assert.True(t, ValidSearchQuery("/PractitionerRole?practitioner=Practitioner/prac-1", constvars.ResourcePractitionerRole, oc))
	// scoped to another practitioner -> denied
	assert.False(t, ValidSearchQuery("/PractitionerRole?practitioner=Practitioner/prac-2", constvars.ResourcePractitionerRole, oc))
}

func TestValidSearchQuery_UnclassifiedTypeDenied(t *testing.T) {
	assert.False(t, ValidSearchQuery("/AdverseEvent?subject=Patient/pat-1", "AdverseEvent", patientOC("pat-1")))
}
