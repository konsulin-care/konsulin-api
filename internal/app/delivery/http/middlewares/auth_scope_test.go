package middlewares

import (
	"konsulin-service/internal/pkg/constvars"
	"testing"

	"github.com/stretchr/testify/assert"
)

// C3: entry-level QuestionnaireResponse/Communication reads must carry an
// identity scope. Aggregate `_summary=count` stays public. Practitioners keep
// their existing authz. Researchers must scope Communication reads to referral
// data (sender/recipient/topic/subject); Superadmin is exempt. Single-resource
// reads (no query) are untouched.

func TestAllowScopedEntryRead_QRCountPublic(t *testing.T) {
	// Aggregate counts are public social-proof data for any caller.
	for _, caller := range [][]string{
		{constvars.KonsulinRoleGuest},
		{constvars.KonsulinRolePatient},
		{constvars.KonsulinRolePractitioner},
	} {
		assert.True(t, allowScopedEntryRead(caller, "pat-1",
			"/QuestionnaireResponse?_summary=count&questionnaire=https://konsulin.care/fhir/Questionnaire/phq2",
			constvars.ResourceQuestionnaireResponse))
	}
}

func TestAllowScopedEntryRead_CommunicationCountPublic(t *testing.T) {
	assert.True(t, allowScopedEntryRead([]string{constvars.KonsulinRoleGuest}, "",
		"/Communication?_summary=count", "Communication"))
}

func TestAllowScopedEntryRead_PatientOwnQRByAuthor(t *testing.T) {
	ok := allowScopedEntryRead([]string{constvars.KonsulinRolePatient}, "pat-1",
		"/QuestionnaireResponse?author=Patient/pat-1&questionnaire=https://konsulin.care/fhir/Questionnaire/phq2&_count=1",
		constvars.ResourceQuestionnaireResponse)
	assert.True(t, ok)
}

func TestAllowScopedEntryRead_PatientOwnQRByPatientParam(t *testing.T) {
	ok := allowScopedEntryRead([]string{constvars.KonsulinRolePatient}, "pat-1",
		"/QuestionnaireResponse?patient=pat-1&questionnaire=https://konsulin.care/fhir/Questionnaire/phq2",
		constvars.ResourceQuestionnaireResponse)
	assert.True(t, ok)
}

func TestAllowScopedEntryRead_PatientRejectedWithoutOwnScope(t *testing.T) {
	// Bare questionnaire search (no author/patient/subject) must be denied.
	ok := allowScopedEntryRead([]string{constvars.KonsulinRolePatient}, "pat-1",
		"/QuestionnaireResponse?questionnaire=https://konsulin.care/fhir/Questionnaire/phq2&_count=1",
		constvars.ResourceQuestionnaireResponse)
	assert.False(t, ok)
}

func TestAllowScopedEntryRead_PatientDeniedAnotherAuthorsQR(t *testing.T) {
	ok := allowScopedEntryRead([]string{constvars.KonsulinRolePatient}, "pat-1",
		"/QuestionnaireResponse?author=Patient/pat-2&questionnaire=https://konsulin.care/fhir/Questionnaire/phq2",
		constvars.ResourceQuestionnaireResponse)
	assert.False(t, ok)
}

func TestAllowScopedEntryRead_GuestOwnQRByIdentifier(t *testing.T) {
	ok := allowScopedEntryRead([]string{constvars.KonsulinRoleGuest}, "",
		"/QuestionnaireResponse?identifier=https://login.konsulin.care/guestid%7Cguest-123",
		constvars.ResourceQuestionnaireResponse)
	assert.True(t, ok)
}

func TestAllowScopedEntryRead_GuestRejectedWithoutIdentifier(t *testing.T) {
	ok := allowScopedEntryRead([]string{constvars.KonsulinRoleGuest}, "",
		"/QuestionnaireResponse?questionnaire=https://konsulin.care/fhir/Questionnaire/phq2&_count=1",
		constvars.ResourceQuestionnaireResponse)
	assert.False(t, ok)
}

func TestAllowScopedEntryRead_GuestNeverReadsCommunication(t *testing.T) {
	ok := allowScopedEntryRead([]string{constvars.KonsulinRoleGuest}, "",
		"/Communication?sender=Patient/pat-1", "Communication")
	assert.False(t, ok)
}

func TestAllowScopedEntryRead_PatientOwnCommunicationBySender(t *testing.T) {
	ok := allowScopedEntryRead([]string{constvars.KonsulinRolePatient}, "pat-1",
		"/Communication?sender=Patient/pat-1&topic=research-referral", "Communication")
	assert.True(t, ok)
}

func TestAllowScopedEntryRead_PatientDeniedOthersCommunication(t *testing.T) {
	ok := allowScopedEntryRead([]string{constvars.KonsulinRolePatient}, "pat-1",
		"/Communication?sender=Patient/pat-2", "Communication")
	assert.False(t, ok)
}

func TestAllowScopedEntryRead_PractitionerQRExempt(t *testing.T) {
	ok := allowScopedEntryRead([]string{constvars.KonsulinRolePractitioner}, "prac-1",
		"/QuestionnaireResponse?patient=pat-2&questionnaire=https://konsulin.care/fhir/Questionnaire/soap",
		constvars.ResourceQuestionnaireResponse)
	assert.True(t, ok)
}

func TestAllowScopedEntryRead_PractitionerCommunicationExempt(t *testing.T) {
	ok := allowScopedEntryRead([]string{constvars.KonsulinRolePractitioner}, "prac-1",
		"/Communication?subject=Patient/pat-2", "Communication")
	assert.True(t, ok)
}

func TestAllowScopedEntryRead_SingleResourceReadExempt(t *testing.T) {
	// Direct single-resource read by id (no query) stays governed by
	// ownership filtering and must not be blocked here.
	ok := allowScopedEntryRead([]string{constvars.KonsulinRoleGuest}, "",
		"/QuestionnaireResponse/qr-123", constvars.ResourceQuestionnaireResponse)
	assert.True(t, ok)
}

func TestAllowScopedEntryRead_OtherResourcesExempt(t *testing.T) {
	ok := allowScopedEntryRead([]string{constvars.KonsulinRoleGuest}, "",
		"/Patient?_id=pat-1", constvars.ResourcePatient)
	assert.True(t, ok)
}

func TestHasRole_Helper(t *testing.T) {
	assert.True(t, hasRole([]string{constvars.KonsulinRolePatient, "x"}, constvars.KonsulinRolePatient))
	assert.False(t, hasRole([]string{constvars.KonsulinRoleGuest}, constvars.KonsulinRolePatient))
}

func TestAllowScopedEntryRead_PatientOwnCommunicationByRecipient(t *testing.T) {
	// Referral writes validate recipient == session patient, so a patient must
	// be able to read Communications scoped to their own recipient id.
	ok := allowScopedEntryRead([]string{constvars.KonsulinRolePatient}, "pat-1",
		"/Communication?recipient=Patient/pat-1&topic=research-referral", constvars.ResourceCommunication)
	assert.True(t, ok)
}

func TestAllowScopedEntryRead_PatientOwnCommunicationByRecipientList(t *testing.T) {
	// FHIR reference search params accept comma-joined values; a patient whose
	// id appears anywhere in the list must pass the ownership scope.
	ok := allowScopedEntryRead([]string{constvars.KonsulinRolePatient}, "pat-1",
		"/Communication?recipient=Patient/pat-2,Patient/pat-1", constvars.ResourceCommunication)
	assert.True(t, ok)
}

func TestAllowScopedEntryRead_PatientDeniedOthersRecipient(t *testing.T) {
	ok := allowScopedEntryRead([]string{constvars.KonsulinRolePatient}, "pat-1",
		"/Communication?recipient=Patient/pat-2", constvars.ResourceCommunication)
	assert.False(t, ok)
}

func TestAllowScopedEntryRead_ResearcherScopedBySender(t *testing.T) {
	ok := allowScopedEntryRead([]string{constvars.KonsulinRoleResearcher}, "",
		"/Communication?sender=Patient/pat-1&topic=research-referral", constvars.ResourceCommunication)
	assert.True(t, ok)
}

func TestAllowScopedEntryRead_ResearcherScopedByTopicOnly(t *testing.T) {
	ok := allowScopedEntryRead([]string{constvars.KonsulinRoleResearcher}, "",
		"/Communication?topic=research-referral", constvars.ResourceCommunication)
	assert.True(t, ok)
}

func TestAllowScopedEntryRead_ResearcherRejectedWithoutScope(t *testing.T) {
	// A bare Communication query by a Researcher must be denied: no scope
	// param means the read is not anchored to any referral data.
	ok := allowScopedEntryRead([]string{constvars.KonsulinRoleResearcher}, "",
		"/Communication?_count=500", constvars.ResourceCommunication)
	assert.False(t, ok)
}

func TestAllowScopedEntryRead_SuperadminExempt(t *testing.T) {
	// Superadmin is trusted and may read unscoped; the response field filter is
	// the abuse guard for that role.
	ok := allowScopedEntryRead([]string{constvars.KonsulinRoleSuperadmin}, "",
		"/Communication?_count=500", constvars.ResourceCommunication)
	assert.True(t, ok)
}

func TestAllowScopedEntryRead_GuestStillDeniedCommunication(t *testing.T) {
	ok := allowScopedEntryRead([]string{constvars.KonsulinRoleGuest}, "",
		"/Communication?topic=research-referral", constvars.ResourceCommunication)
	assert.False(t, ok)
}
