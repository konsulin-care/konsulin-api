package constvars

// FHIR search parameter names used by the purge ownership registry.
const (
	PurgeSearchParamAuthor  = "author"
	PurgeSearchParamSender  = "sender"
	PurgeSearchParamPatient = "patient"
	PurgeSearchParamSubject = "subject"
	PurgeSearchParamActor   = "actor"
)

// PurgeRule expresses one FHIR resource type whose active ownership by a
// patient warrants full deletion during the erasure (purge) flow. Params lists
// the search parameters that prove active ownership (e.g. QuestionnaireResponse
// is actively owned via its author, not its subject).
type PurgeRule struct {
	ResourceType string
	Params       []string
}

// PurgeRules is the single source of truth for which resource types are
// deleted during a patient purge and which search parameters prove active
// ownership. Passive references (subject, recipient, study) are intentionally
// absent: those resources are kept and simply point at the retained Patient
// shell. Shared types like ResearchStudy, PlanDefinition, and Questionnaire
// are never enumerated here, so they are excluded from the delete path by
// construction.
var PurgeRules = []PurgeRule{
	{ResourceType: ResourceQuestionnaireResponse, Params: []string{PurgeSearchParamAuthor}},
	{ResourceType: ResourceCommunication, Params: []string{PurgeSearchParamSender}},
	{ResourceType: ResourceConsent, Params: []string{PurgeSearchParamPatient}},
	{ResourceType: ResourceResearchSubject, Params: []string{PurgeSearchParamPatient}},
	{ResourceType: ResourceObservation, Params: []string{PurgeSearchParamSubject}},
	{ResourceType: ResourceCondition, Params: []string{PurgeSearchParamSubject}},
	{ResourceType: ResourceAppointment, Params: []string{PurgeSearchParamActor}},
}
