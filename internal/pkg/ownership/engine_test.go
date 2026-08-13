package ownership

import (
	"testing"

	"konsulin-service/internal/pkg/constvars"

	"github.com/stretchr/testify/assert"
)

// testOC builds an OwnershipContext with the given identities.
func testOC() *OwnershipContext {
	return &OwnershipContext{
		PatientIDs:              map[string]struct{}{},
		PractitionerIDs:         map[string]struct{}{},
		PractitionerRoleIDs:     map[string]struct{}{},
		PractitionerRoleCodings: map[string]struct{}{},
		PersonIDs:               map[string]struct{}{},
	}
}

func patientOC(id string) *OwnershipContext {
	oc := testOC()
	oc.HasPatientRole = true
	oc.PatientIDs[id] = struct{}{}
	return oc
}

func practitionerOC(id string, codings ...string) *OwnershipContext {
	oc := testOC()
	oc.HasPractitionerRole = true
	oc.PractitionerIDs[id] = struct{}{}
	for _, c := range codings {
		oc.PractitionerRoleCodings[c] = struct{}{}
	}
	return oc
}

func assertOwned(t *testing.T, raw, resourceType string, oc *OwnershipContext, want bool) {
	t.Helper()
	got, err := OwnedBy([]byte(raw), resourceType, oc)
	assert.NoError(t, err)
	assert.Equal(t, want, got, "OwnedBy(%s)", resourceType)
}

func TestOwnedBy_UnclassifiedTypeFailsClosed(t *testing.T) {
	// The fail-closed flip: a resource type with no rule is denied for any caller.
	oc := patientOC("pat-1")
	assertOwned(t, `{"resourceType":"AdverseEvent","subject":{"reference":"Patient/pat-1"}}`, "AdverseEvent", oc, false)
	assertOwned(t, `{"resourceType":"AdverseEvent","subject":{"reference":"Patient/pat-1"}}`, "AdverseEvent", testOC(), false)
}

func TestOwnedBy_PublicResourceAlwaysAllowed(t *testing.T) {
	oc := testOC()
	assertOwned(t, `{"resourceType":"Location","id":"loc-1"}`, constvars.ResourceLocation, oc, true)
	assertOwned(t, `{"resourceType":"Questionnaire","id":"q-1"}`, constvars.ResourceQuestionnaire, oc, true)
	assertOwned(t, `{"resourceType":"PlanDefinition","id":"pd-1"}`, "PlanDefinition", oc, true)
	assertOwned(t, `{"resourceType":"metadata"}`, "metadata", oc, true)
}

func TestOwnedBy_InternalScopeDenied(t *testing.T) {
	// Internal is reserved for gateway-only access; FHIR proxy denies it.
	oc := testOC()
	assertOwned(t, `{"resourceType":"PaymentReconciliation","id":"pr-1"}`, "PaymentReconciliation", oc, false)
}

func TestOwnedBy_PatientOwnsViaSubject(t *testing.T) {
	oc := patientOC("pat-1")
	assertOwned(t, `{"resourceType":"Condition","id":"c-1","subject":{"reference":"Patient/pat-1"}}`, constvars.ResourceCondition, oc, true)
	// other patient's condition denied
	assertOwned(t, `{"resourceType":"Condition","id":"c-2","subject":{"reference":"Patient/pat-2"}}`, constvars.ResourceCondition, oc, false)
	// no subject at all denied
	assertOwned(t, `{"resourceType":"Condition","id":"c-3"}`, constvars.ResourceCondition, oc, false)
}

func TestOwnedBy_PatientOwnsOwnResourceByID(t *testing.T) {
	oc := patientOC("pat-1")
	assertOwned(t, `{"resourceType":"Patient","id":"pat-1"}`, constvars.ResourcePatient, oc, true)
	assertOwned(t, `{"resourceType":"Patient","id":"pat-2"}`, constvars.ResourcePatient, oc, false)
}

func TestOwnedBy_PatientOwnsViaArrayRefs(t *testing.T) {
	oc := patientOC("pat-1")
	appt := `{"resourceType":"Appointment","id":"a-1","participant":[{"actor":{"reference":"Practitioner/prac-1"}},{"actor":{"reference":"Patient/pat-1"}}]}`
	assertOwned(t, appt, constvars.ResourceAppointment, oc, true)
	other := `{"resourceType":"Appointment","id":"a-2","participant":[{"actor":{"reference":"Practitioner/prac-1"}}]}`
	assertOwned(t, other, constvars.ResourceAppointment, oc, false)
}

func TestOwnedBy_PractitionerOwnsViaActor(t *testing.T) {
	oc := practitionerOC("prac-1")
	enc := `{"resourceType":"Encounter","id":"e-1","practitioner":[{"reference":"Practitioner/prac-1"}]}`
	assertOwned(t, enc, constvars.ResourceEncounter, oc, true)
	assertOwned(t, `{"resourceType":"Encounter","id":"e-2","practitioner":[{"reference":"Practitioner/prac-2"}]}`, constvars.ResourceEncounter, oc, false)
}

func TestOwnedBy_PractitionerOwnsOwnResourceByID(t *testing.T) {
	oc := practitionerOC("prac-1")
	assertOwned(t, `{"resourceType":"Practitioner","id":"prac-1"}`, constvars.ResourcePractitioner, oc, true)
	assertOwned(t, `{"resourceType":"Practitioner","id":"prac-2"}`, constvars.ResourcePractitioner, oc, false)
}

func TestOwnedBy_PractitionerOwnsViaPractitionerRoleID(t *testing.T) {
	oc := practitionerOC("prac-1")
	oc.PractitionerRoleIDs["role-1"] = struct{}{}
	assertOwned(t, `{"resourceType":"Schedule","id":"s-1","actor":[{"reference":"PractitionerRole/role-1"}]}`, constvars.ResourceSchedule, oc, true)
}

func TestOwnedBy_SharedCommunicationSenderOrRecipient(t *testing.T) {
	patient := patientOC("pat-1")
	assertOwned(t, `{"resourceType":"Communication","id":"m-1","sender":{"reference":"Patient/pat-1"}}`, constvars.ResourceCommunication, patient, true)
	assertOwned(t, `{"resourceType":"Communication","id":"m-2","recipient":{"reference":"Patient/pat-1"}}`, constvars.ResourceCommunication, patient, true)
	assertOwned(t, `{"resourceType":"Communication","id":"m-3","sender":{"reference":"Practitioner/prac-9"}}`, constvars.ResourceCommunication, patient, false)

	prac := practitionerOC("prac-1")
	assertOwned(t, `{"resourceType":"Communication","id":"m-4","sender":{"reference":"Practitioner/prac-1"}}`, constvars.ResourceCommunication, prac, true)
	assertOwned(t, `{"resourceType":"Communication","id":"m-5","recipient":{"reference":"Practitioner/prac-1"}}`, constvars.ResourceCommunication, prac, true)
	assertOwned(t, `{"resourceType":"Communication","id":"m-6","sender":{"reference":"Patient/pat-9"}}`, constvars.ResourceCommunication, prac, false)
}

func TestOwnedBy_ResearcherReadsCommunicationWithCoding(t *testing.T) {
	researcher := practitionerOC("prac-1", constvars.FhirPractitionerRoleSystemHL7+"|"+constvars.FhirPractitionerRoleCodeResearcher)
	// researcher reads a communication by topic (no own sender/recipient ref)
	assertOwned(t, `{"resourceType":"Communication","id":"m-1","topic":{"text":"research-referral"},"sender":{"reference":"Patient/pat-9"}}`, constvars.ResourceCommunication, researcher, true)
	// plain practitioner without the coding cannot read a stranger's communication
	plain := practitionerOC("prac-1")
	assertOwned(t, `{"resourceType":"Communication","id":"m-1","topic":{"text":"research-referral"},"sender":{"reference":"Patient/pat-9"}}`, constvars.ResourceCommunication, plain, false)
}

func TestOwnedBy_ClinicAdminCodingAllowsPersonRead(t *testing.T) {
	admin := practitionerOC("prac-1", constvars.FhirPractitionerRoleSystemSnomed+"|"+constvars.FhirPractitionerRoleCodeAdministrativeStaff)
	admin.PersonIDs["person-1"] = struct{}{}
	assertOwned(t, `{"resourceType":"Person","id":"person-1"}`, constvars.ResourcePerson, admin, true)
	other := practitionerOC("prac-2", constvars.FhirPractitionerRoleSystemSnomed+"|"+constvars.FhirPractitionerRoleCodeAdministrativeStaff)
	assertOwned(t, `{"resourceType":"Person","id":"person-1"}`, constvars.ResourcePerson, other, false)
}

func TestOwnedBy_InvoiceCheckerAllowsWhitelistedRefs(t *testing.T) {
	oc := testOC()
	// every reference points at whitelisted actors -> public invoice
	invoice := `{"resourceType":"Invoice","id":"inv-1","participant":[{"actor":{"reference":"PractitionerRole/role-1"}},{"actor":{"reference":"Device/dev-1"}}]}`
	assertOwned(t, invoice, constvars.ResourceInvoice, oc, true)
	// an invoice referencing a patient is not public; owned only via matching refs
	patientInvoice := `{"resourceType":"Invoice","id":"inv-2","subject":{"reference":"Patient/pat-1"}}`
	assertOwned(t, patientInvoice, constvars.ResourceInvoice, testOC(), false)
	assertOwned(t, patientInvoice, constvars.ResourceInvoice, patientOC("pat-1"), true)
}

func TestOwnedBy_ObservationSharedPatientAndPractitioner(t *testing.T) {
	patient := patientOC("pat-1")
	assertOwned(t, `{"resourceType":"Observation","id":"o-1","subject":{"reference":"Patient/pat-1"}}`, constvars.ResourceObservation, patient, true)

	prac := practitionerOC("prac-1")
	assertOwned(t, `{"resourceType":"Observation","id":"o-2","performer":[{"reference":"Practitioner/prac-1"}]}`, constvars.ResourceObservation, prac, true)
	assertOwned(t, `{"resourceType":"Observation","id":"o-3","subject":{"reference":"Patient/pat-9"}}`, constvars.ResourceObservation, prac, false)
}

func TestOwnedBy_MalformedJSONDenies(t *testing.T) {
	oc := patientOC("pat-1")
	got, err := OwnedBy([]byte(`{not json`), constvars.ResourceCondition, oc)
	assert.Error(t, err)
	assert.False(t, got)
}
