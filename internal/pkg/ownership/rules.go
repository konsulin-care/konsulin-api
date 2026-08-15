package ownership

import "konsulin-service/internal/pkg/constvars"

// Rules is the declarative ownership specification: one entry per FHIR
// resource type. It is the single source of truth for read ownership, write
// body validation, and search-query scoping. Adding a new resource type means
// adding one entry here (plus, for exotic types, a checker in checkers.go or a
// named write checker in the middleware layer).
//
// Reference paths are gjson paths into the FHIR JSON (e.g.
// "subject.reference", "participant.#.actor.reference"). The compartment
// conformance test (compartment_test.go) asserts every Patient/Practitioner
// ref exists in the vendored FHIR R4 CompartmentDefinitions.
//
//nolint:goconst // the rules table is inherently literal resource-type data
var Rules = map[string]ResourceRule{
	// ------------------------------------------------------------------ Public
	// Catalog and system resources: readable by any caller without ownership
	// proof. Never carry Refs.
	"metadata":                          publicRule("metadata"),
	constvars.ResourceHealthcareService: publicRule(constvars.ResourceHealthcareService),
	constvars.ResourceLocation:          publicRule(constvars.ResourceLocation),
	"Media":                             publicRule("Media"),
	constvars.ResourceOrganization:      publicRule(constvars.ResourceOrganization),
	"PlanDefinition":                    publicRule("PlanDefinition"),
	constvars.ResourceQuestionnaire:     publicRule(constvars.ResourceQuestionnaire),
	"ResearchStudy":                     publicRule("ResearchStudy"),

	// PractitionerRole is public listing, but a search carrying a practitioner
	// or actor param must reference an owned practitioner.
	constvars.ResourcePractitionerRole: {
		ResourceType: constvars.ResourcePractitionerRole,
		Scope:        ScopePublic,
		SearchParams: []string{"practitioner", "actor"},
		// A plain practitioner may only PUT a role bound to themselves; a
		// clinic admin manages roles for other practitioners.
		WriteRefs:        []Ref{pracRef("practitioner.reference")},
		WriteBypassCodes: []string{CodingClinicAdmin},
	},

	// Schedule and Slot are public availability data on reads; writes carry
	// legacy I/O checks via named write checkers.
	constvars.ResourceSchedule: {
		ResourceType:     constvars.ResourceSchedule,
		Scope:            ScopePublic,
		SearchParams:     []string{"actor"},
		WriteCheckerName: "schedule",
	},
	constvars.ResourceSlot: {
		ResourceType:     constvars.ResourceSlot,
		Scope:            ScopePublic,
		WriteCheckerName: "slot",
	},

	// Legacy public metadata/terminology types (previously IsPublicResource).
	"CodeSystem":              publicRule("CodeSystem"),
	"ValueSet":                publicRule("ValueSet"),
	"ConceptMap":              publicRule("ConceptMap"),
	"StructureDefinition":     publicRule("StructureDefinition"),
	"OperationDefinition":     publicRule("OperationDefinition"),
	"SearchParameter":         publicRule("SearchParameter"),
	"CompartmentDefinition":   publicRule("CompartmentDefinition"),
	"GraphDefinition":         publicRule("GraphDefinition"),
	"ImplementationGuide":     publicRule("ImplementationGuide"),
	"CapabilityStatement":     publicRule("CapabilityStatement"),
	"MessageDefinition":       publicRule("MessageDefinition"),
	"ActivityDefinition":      publicRule("ActivityDefinition"),
	"Library":                 publicRule("Library"),
	"Measure":                 publicRule("Measure"),
	"MeasureReport":           publicRule("MeasureReport"),
	"TestScript":              publicRule("TestScript"),
	"TestReport":              publicRule("TestReport"),
	"Subscription":            publicRule("Subscription"),
	"SubscriptionTopic":       publicRule("SubscriptionTopic"),
	"VerificationResult":      publicRule("VerificationResult"),
	"Requirements":            publicRule("Requirements"),
	"ExampleScenario":         publicRule("ExampleScenario"),
	"SpecimenDefinition":      publicRule("SpecimenDefinition"),
	"NamingSystem":            publicRule("NamingSystem"),
	"TerminologyCapabilities": publicRule("TerminologyCapabilities"),

	// ------------------------------------------------------------------ Patient
	constvars.ResourcePatient: {
		ResourceType: constvars.ResourcePatient,
		Scope:        ScopePatient,
		Refs:         []Ref{idRef(constvars.ResourcePatient)},
		WriteRefs:    []Ref{idRef(constvars.ResourcePatient)},
		SearchParams: []string{"_id"},
	},
	constvars.ResourceCondition: {
		ResourceType: constvars.ResourceCondition,
		Scope:        ScopePatient,
		Refs:         []Ref{pRef(refSubject)},
		WriteRefs:    []Ref{pRef(refSubject)},
		SearchParams: []string{"patient", "subject"},
	},
	constvars.ResourceConsent: {
		ResourceType: constvars.ResourceConsent,
		Scope:        ScopePatient,
		Refs:         []Ref{pRef(refPatient)},
		WriteRefs:    []Ref{pRef(refPatient)},
		SearchParams: []string{"patient"},
	},
	constvars.ResourceResearchSubject: {
		ResourceType: constvars.ResourceResearchSubject,
		Scope:        ScopePatient,
		Refs:         []Ref{pRef("individual.reference")},
		WriteRefs:    []Ref{pRef("individual.reference")},
		SearchParams: []string{"individual"},
	},
	"AllergyIntolerance": {
		ResourceType: "AllergyIntolerance",
		Scope:        ScopePatient,
		Refs:         []Ref{pRef(refPatient), pRef("recorder.reference"), pRef("asserter.reference")},
		SearchParams: []string{"patient"},
	},
	"FamilyMemberHistory": {
		ResourceType: "FamilyMemberHistory",
		Scope:        ScopePatient,
		Refs:         []Ref{pRef(refPatient)},
		SearchParams: []string{"patient"},
	},
	"Goal": {
		ResourceType: "Goal",
		Scope:        ScopePatient,
		Refs:         []Ref{pRef(refSubject)},
		SearchParams: []string{"patient"},
	},
	"ImagingStudy": {
		ResourceType: "ImagingStudy",
		Scope:        ScopePatient,
		Refs:         []Ref{pRef(refPatient)},
		SearchParams: []string{"patient"},
	},
	"Immunization": {
		ResourceType: "Immunization",
		Scope:        ScopePatient,
		Refs:         []Ref{pRef(refPatient)},
		SearchParams: []string{"patient"},
	},
	"RiskAssessment": {
		ResourceType: "RiskAssessment",
		Scope:        ScopePatient,
		Refs:         []Ref{pRef(refSubject)},
		SearchParams: []string{"patient"},
	},
	"MedicationDispense": {
		ResourceType: "MedicationDispense",
		Scope:        ScopePatient,
		Refs:         []Ref{pRef(refSubject), pRef(refPatient), pRef("receiver.reference")},
		SearchParams: []string{"patient"},
	},
	"MedicationStatement": {
		ResourceType: "MedicationStatement",
		Scope:        ScopePatient,
		Refs:         []Ref{pRef(refSubject)},
		SearchParams: []string{"patient"},
	},
	"Coverage": {
		ResourceType: "Coverage",
		Scope:        ScopePatient,
		Refs: []Ref{
			pRef("policy-holder.reference"), pRef("subscriber.reference"),
			pRef("beneficiary.reference"), pRef("payor.#.reference"),
		},
		SearchParams: []string{"patient"},
	},

	// ------------------------------------------------------------ Shared
	// Owned by any identity a ref names: patient via one side, practitioner via
	// the other.
	constvars.ResourceAppointment: {
		ResourceType: constvars.ResourceAppointment,
		Scope:        ScopeShared,
		Refs: []Ref{
			pRef(refActor),
			pracRef(refActor),
			roleRef(refActor),
		},
		WriteRefs:    []Ref{pRef(refActor)},
		SearchParams: []string{"patient", "subject", "actor", "practitioner", "participant"},
	},
	constvars.ResourceCommunication: {
		ResourceType: constvars.ResourceCommunication,
		Scope:        ScopeShared,
		Refs: []Ref{
			pRef(refSender), pRef(refRecipient), pRef(refSubject),
			pracRef(refSender), pracRef(refRecipient),
		},
		// Researchers may read Communications anchored to referral data; such
		// reads are redacted to RedactKeep.
		CodeAllow: []string{CodingResearcher},
		RedactKeep: []string{
			"resourceType", "id", "meta", "sender", "recipient", "sent", "received",
		},
		SearchParams: []string{"sender", "recipient"},
		SearchAllowances: []SearchAllowance{{
			PractitionerRoleCodings: []string{CodingResearcher},
			Params:                  []string{"sender", "recipient", "topic", "subject"},
		}},
		WriteRefs: []Ref{pRef(refSender)},
	},
	constvars.ResourceEncounter: {
		ResourceType: constvars.ResourceEncounter,
		Scope:        ScopeShared,
		Refs: []Ref{
			pRef(refPatient),
			pracRef("practitioner.#.reference"), pracRef("participant.#.individual.reference"),
		},
		SearchParams: []string{"patient"},
	},
	constvars.ResourceObservation: {
		ResourceType: constvars.ResourceObservation,
		Scope:        ScopeShared,
		Refs: []Ref{
			pRef(refSubject),
			pracRef(refPerformer),
		},
		WriteRefs:    []Ref{pRef(refSubject)},
		SearchParams: []string{"patient", "subject", "performer"},
	},
	constvars.ResourceQuestionnaireResponse: {
		ResourceType: constvars.ResourceQuestionnaireResponse,
		Scope:        ScopeShared,
		Refs: []Ref{
			pRef(refAuthor), pRef(refSubject),
			pracRef(refAuthor), pracRef("source.reference"),
		},
		SearchParams: []string{"author", "patient", "subject"},
		SearchAllowances: []SearchAllowance{{
			PractitionerRoleCodings: []string{CodingResearcher},
			Params:                  []string{"identifier"},
		}},
		WriteCheckerName: "questionnaire_response",
	},
	constvars.ResourceInvoice: {
		ResourceType: constvars.ResourceInvoice,
		Scope:        ScopeShared,
		Refs: []Ref{
			pRef(refSubject), pRef(refPatient), pRef(refRecipient),
			pracRef(refActor), roleRef(refActor),
		},
		Checker:          invoiceChecker,
		WriteBypassCodes: []string{CodingClinicAdmin},
		WriteCheckerName: "invoice",
		SearchParams:     []string{"patient"},
	},
	"CarePlan": {
		ResourceType: "CarePlan",
		Scope:        ScopeShared,
		Refs:         []Ref{pRef(refSubject), pracRef("performer.#.actor.reference")},
		SearchParams: []string{"patient"},
	},
	"DiagnosticReport": {
		ResourceType: "DiagnosticReport",
		Scope:        ScopeShared,
		Refs:         []Ref{pRef(refSubject), pracRef(refPerformer)},
		SearchParams: []string{"patient", "subject"},
	},
	"DocumentReference": {
		ResourceType: "DocumentReference",
		Scope:        ScopeShared,
		Refs:         []Ref{pRef(refSubject), pracRef("author.#.reference")},
		SearchParams: []string{"patient", "subject"},
	},
	"Procedure": {
		ResourceType: "Procedure",
		Scope:        ScopeShared,
		Refs:         []Ref{pRef(refSubject), pracRef(refPerformer)},
		SearchParams: []string{"patient", "subject"},
	},
	"MedicationRequest": {
		ResourceType: "MedicationRequest",
		Scope:        ScopeShared,
		Refs:         []Ref{pRef(refSubject), pracRef("requester.reference")},
		SearchParams: []string{"patient", "subject"},
	},
	"MedicationAdministration": {
		ResourceType: "MedicationAdministration",
		Scope:        ScopeShared,
		Refs:         []Ref{pRef(refPatient), pRef(refSubject), pracRef("performer.#.actor.reference")},
		SearchParams: []string{"patient"},
	},
	"Claim": {
		ResourceType: "Claim",
		Scope:        ScopeShared,
		Refs:         []Ref{pRef(refPatient), pracRef(refEnterer), pracRef(refProvider)},
		SearchParams: []string{"patient"},
	},
	"ExplanationOfBenefit": {
		ResourceType: "ExplanationOfBenefit",
		Scope:        ScopeShared,
		Refs:         []Ref{pRef(refPatient), pracRef(refEnterer), pracRef(refProvider)},
		SearchParams: []string{"patient"},
	},
	"CoverageEligibilityRequest": {
		ResourceType: "CoverageEligibilityRequest",
		Scope:        ScopeShared,
		Refs:         []Ref{pRef(refPatient), pracRef(refProvider), pracRef(refEnterer)},
		SearchParams: []string{"patient"},
	},
	"CoverageEligibilityResponse": {
		ResourceType: "CoverageEligibilityResponse",
		Scope:        ScopeShared,
		Refs:         []Ref{pRef(refPatient), pracRef("requestor.reference")},
		SearchParams: []string{"patient"},
	},
	"ClaimResponse": {
		ResourceType: "ClaimResponse",
		Scope:        ScopeShared,
		Refs:         []Ref{pRef(refPatient), pracRef("requestor.reference")},
		SearchParams: []string{"patient"},
	},
	"Account": {
		ResourceType: "Account",
		Scope:        ScopeShared,
		Refs:         []Ref{pRef(refSubject)},
		SearchParams: []string{"patient"},
	},
	"ChargeItem": {
		ResourceType: "ChargeItem",
		Scope:        ScopeShared,
		Refs:         []Ref{pRef(refSubject), pracRef(refEnterer)},
		SearchParams: []string{"patient"},
	},
	"PaymentNotice": {
		ResourceType: "PaymentNotice",
		Scope:        ScopeShared,
		Refs:         []Ref{pRef(refProvider), pracRef(refProvider)},
		SearchParams: []string{"patient"},
	},
	"CommunicationRequest": {
		ResourceType: "CommunicationRequest",
		Scope:        ScopeShared,
		Refs: []Ref{
			pRef(refSubject), pRef(refSender), pRef("recipient.#.reference"),
			pracRef("requester.reference"),
		},
		SearchParams: []string{"patient", "sender", "recipient"},
	},

	// ----------------------------------------------------------- Practitioner
	constvars.ResourcePractitioner: {
		ResourceType: constvars.ResourcePractitioner,
		Scope:        ScopePractitioner,
		Refs:         []Ref{idRef(constvars.ResourcePractitioner)},
		// Clinic admins manage practitioners; they may read any Practitioner.
		CodeAllow:    []string{CodingClinicAdmin},
		WriteRefs:    []Ref{idRef(constvars.ResourcePractitioner)},
		SearchParams: []string{"_id"},
	},
	"Task": {
		ResourceType: "Task",
		Scope:        ScopePractitioner,
		Refs:         []Ref{pracRef("owner.reference")},
	},
	"Contract": {
		ResourceType: "Contract",
		Scope:        ScopePractitioner,
		Refs:         []Ref{pracRef(refAuthor)},
	},

	// ---------------------------------------------------------------- Person
	constvars.ResourcePerson: {
		ResourceType:            constvars.ResourcePerson,
		Scope:                   ScopePerson,
		Refs:                    []Ref{personRef("id")},
		PractitionerRoleCodings: []string{CodingClinicAdmin},
		SearchParams:            []string{"patient", "practitioner"},
	},

	// ---------------------------------------------------------------- Internal
	// Gateway-only resources, deliberately not reachable through the FHIR
	// proxy ownership model.
	"PaymentReconciliation": {ResourceType: "PaymentReconciliation", Scope: ScopeInternal},
}
