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

func TestValidWriteQuery_ScopedTypeRequiresOwnedQuery(t *testing.T) {
	oc := patientOC("pat-1")
	// unowned patient in query -> denied
	assert.False(t, ValidWriteQuery("/fhir/Condition?patient=pat-2", constvars.ResourceCondition, oc))
	// owned patient in query -> allowed
	assert.True(t, ValidWriteQuery("/fhir/Condition?patient=pat-1", constvars.ResourceCondition, oc))
	// owned via subject with Patient/ prefix
	assert.True(t, ValidWriteQuery("/fhir/Condition?subject=Patient/pat-1", constvars.ResourceCondition, oc))
	// non-ownership param only (_id is not a Condition scoping param) -> denied
	assert.False(t, ValidWriteQuery("/fhir/Condition?_id=cond-123", constvars.ResourceCondition, oc))
	// bare listing -> denied
	assert.False(t, ValidWriteQuery("/fhir/Condition?_count=50", constvars.ResourceCondition, oc))
}

func TestValidWriteQuery_NoQueryDeniedForScopedTypes(t *testing.T) {
	// A single-resource DELETE on a scoped type cannot prove ownership without
	// a scoping query; deny (fail-closed).
	oc := patientOC("pat-1")
	assert.False(t, ValidWriteQuery("/fhir/Condition/cond-123", constvars.ResourceCondition, oc))
	assert.False(t, ValidWriteQuery("/fhir/Encounter/enc-123", constvars.ResourceEncounter, oc))
	assert.False(t, ValidWriteQuery("/fhir/Appointment/appt-123", constvars.ResourceAppointment, oc))
}

func TestValidWriteQuery_IdentityTypeOwnedPathOrID(t *testing.T) {
	oc := patientOC("pat-1")
	assert.True(t, ValidWriteQuery("/fhir/Patient/pat-1", constvars.ResourcePatient, oc))
	assert.False(t, ValidWriteQuery("/fhir/Patient/pat-2", constvars.ResourcePatient, oc))
	assert.True(t, ValidWriteQuery("/fhir/Patient?_id=pat-1", constvars.ResourcePatient, oc))
	assert.False(t, ValidWriteQuery("/fhir/Patient?_id=pat-2", constvars.ResourcePatient, oc))
	// no path id and no query -> denied
	assert.False(t, ValidWriteQuery("/fhir/Patient", constvars.ResourcePatient, oc))

	prac := practitionerOC("prac-1")
	assert.True(t, ValidWriteQuery("/fhir/Practitioner/prac-1", constvars.ResourcePractitioner, prac))
	assert.False(t, ValidWriteQuery("/fhir/Practitioner/prac-2", constvars.ResourcePractitioner, prac))
	assert.True(t, ValidWriteQuery("/fhir/Practitioner?_id=prac-1", constvars.ResourcePractitioner, prac))
}

func TestValidWriteQuery_AllValuesMustBeOwned(t *testing.T) {
	oc := patientOC("pat-1")
	// one owned value plus one unowned value in the same param -> denied
	assert.False(t, ValidWriteQuery("/fhir/Observation?patient=pat-1,pat-2", constvars.ResourceObservation, oc))
	assert.False(t, ValidWriteQuery("/fhir/Observation?patient=pat-1&subject=Patient/pat-2", constvars.ResourceObservation, oc))
	assert.True(t, ValidWriteQuery("/fhir/Observation?patient=pat-1&subject=Patient/pat-1", constvars.ResourceObservation, oc))
}

func TestValidWriteQuery_SharedTypeScopedByAnyOwnershipParam(t *testing.T) {
	oc := patientOC("pat-1")
	assert.True(t, ValidWriteQuery("/fhir/Appointment?patient=pat-1", constvars.ResourceAppointment, oc))
	assert.False(t, ValidWriteQuery("/fhir/Appointment?patient=pat-2", constvars.ResourceAppointment, oc))

	prac := practitionerOC("prac-1")
	assert.True(t, ValidWriteQuery("/fhir/Appointment?actor=Practitioner/prac-1", constvars.ResourceAppointment, prac))
	assert.False(t, ValidWriteQuery("/fhir/Appointment?actor=Practitioner/prac-2", constvars.ResourceAppointment, prac))
}

func TestValidWriteQuery_PublicTypeStrictScoping(t *testing.T) {
	oc := practitionerOC("prac-1")
	// PractitionerRole is public with SearchParams: a mutating query must carry
	// an ownership param referencing an owned identity.
	assert.True(t, ValidWriteQuery("/fhir/PractitionerRole?practitioner=Practitioner/prac-1", constvars.ResourcePractitionerRole, oc))
	assert.False(t, ValidWriteQuery("/fhir/PractitionerRole?practitioner=Practitioner/prac-2", constvars.ResourcePractitionerRole, oc))
	// no ownership param -> denied (no bare mutating listing)
	assert.False(t, ValidWriteQuery("/fhir/PractitionerRole?_count=50", constvars.ResourcePractitionerRole, oc))
}

func TestValidWriteQuery_InternalAndUnknownTypesDenied(t *testing.T) {
	oc := testOC()
	assert.False(t, ValidWriteQuery("/fhir/PaymentReconciliation/pr-1", "PaymentReconciliation", oc))
	assert.False(t, ValidWriteQuery("/fhir/AdverseEvent?subject=Patient/pat-1", "AdverseEvent", patientOC("pat-1")))
}

func TestValidWriteQuery_ScopedEntryRequiresAnchoredScope(t *testing.T) {
	// A plain practitioner gets no exemption on the mutating path: an unanchored
	// or unowned scoped-entry query is denied.
	prac := practitionerOC("prac-1")
	assert.False(t, ValidWriteQuery("/fhir/QuestionnaireResponse?patient=pat-2", constvars.ResourceQuestionnaireResponse, prac))
	assert.False(t, ValidWriteQuery("/fhir/Communication?topic=research-referral", constvars.ResourceCommunication, prac))

	// patient own-ref anchors the scope
	pat := patientOC("pat-1")
	assert.True(t, ValidWriteQuery("/fhir/QuestionnaireResponse?author=Patient/pat-1", constvars.ResourceQuestionnaireResponse, pat))
	assert.True(t, ValidWriteQuery("/fhir/Communication?sender=Patient/pat-1&topic=research-referral", constvars.ResourceCommunication, pat))

	// researcher allowance anchors the scope
	researcher := practitionerOC("prac-1", constvars.FhirPractitionerRoleSystemHL7+"|"+constvars.FhirPractitionerRoleCodeResearcher)
	assert.True(t, ValidWriteQuery("/fhir/Communication?topic=research-referral", constvars.ResourceCommunication, researcher))
	assert.False(t, ValidWriteQuery("/fhir/Communication?_count=500", constvars.ResourceCommunication, researcher))

	// QR by identifier stays anchored for non-patient callers
	assert.True(t, ValidWriteQuery("/fhir/QuestionnaireResponse?identifier=https://login.konsulin.care/guestid%7Cguest-123", constvars.ResourceQuestionnaireResponse, prac))
}
