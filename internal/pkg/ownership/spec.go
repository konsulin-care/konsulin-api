package ownership

import "konsulin-service/internal/pkg/constvars"

// Scope classifies how a FHIR resource type is gated by the ownership engine.
type Scope int

const (
	// ScopePublic: readable by any caller without ownership proof (catalog and
	// system resources). No Refs.
	ScopePublic Scope = iota
	// ScopePatient: owned by a patient identity via its reference elements.
	ScopePatient
	// ScopePractitioner: owned by a practitioner identity via its references.
	ScopePractitioner
	// ScopePerson: owned by a Person identity (linked patient/practitioner).
	// No session resolves to a Person identity yet; Person reads fail closed.
	ScopePerson
	// ScopeShared: owned by any identity its refs name (multi-owner), e.g.
	// Communication (sender OR recipient), Encounter (subject OR participant).
	ScopeShared
	// ScopeInternal: deliberately not reachable through the ownership model.
	// Denied for all callers on the FHIR proxy; gateway endpoints carry their
	// own authorization.
	ScopeInternal
)

// Ref describes one reference element that confers ownership: when the JSON
// value at Path (a gjson path, e.g. "subject.reference" or
// "participant.#.actor.reference") points at an identity of Target type that
// the caller owns, the resource is owned. Path "id" matches the resource's own
// id element (identity resources like Patient/Practitioner/Person).
type Ref struct {
	Path   string
	Target string // constvars.ResourcePatient | ResourcePractitioner | ResourcePractitionerRole | ResourcePerson
}

// SearchAllowance is a code-conditioned search allowance: callers holding one
// of the codings may run entry-level searches carrying any of the listed
// params (the params anchor the read to the caller's domain, e.g. a researcher
// reading Communications by topic).
type SearchAllowance struct {
	PractitionerRoleCodings []string
	Params                  []string
}

// OwnershipChecker is a per-resource-type strategy evaluated after the
// declarative Refs fail to match (e.g. Invoice's "public when every ref is a
// whitelisted actor" rule).
type OwnershipChecker func(raw []byte, oc *OwnershipContext) (bool, error)

// ResourceRule is the declarative ownership specification for one FHIR
// resource type. It is the single source of truth for read ownership, write
// body validation, and search-query scoping.
type ResourceRule struct {
	ResourceType string
	Scope        Scope
	// Refs confer read ownership (see Ref).
	Refs []Ref
	// CodeAllow lists practitioner-role codings that permit non-owner reads of
	// the resource (e.g. a researcher reading Communications by topic). Reads
	// via CodeAllow are subject to RedactKeep field reduction.
	CodeAllow []string
	// RedactKeep lists fields kept when a non-owner read is redacted. Empty
	// means no redaction.
	RedactKeep []string
	// WriteRefs are the reference elements a write body must carry to prove
	// ownership (e.g. Condition.subject must reference the caller's own
	// Patient). Enforced by ValidateWriteBody.
	WriteRefs []Ref
	// WriteBypassCodes list codings whose holders are exempt from WriteRefs
	// (e.g. a clinic admin manages PractitionerRole resources for others).
	WriteBypassCodes []string
	// SearchParams are query params that must reference an owned identity when
	// present on a search (e.g. PractitionerRole?practitioner=...).
	SearchParams []string
	// SearchAllowances are code-conditioned entry-level search allowances.
	SearchAllowances []SearchAllowance
	// PractitionerRoleCodings, when non-empty, restrict the whole rule's
	// ownership to callers holding one of the codings.
	PractitionerRoleCodings []string
	// WriteCheckerName names a middleware-registered write strategy for types
	// whose write validation needs I/O or legacy quirks (schedule, invoice,
	// slot, questionnaire_response).
	WriteCheckerName string
	// Checker is a read-path strategy evaluated after Refs (invoice).
	Checker OwnershipChecker
}

// publicRule builds a ScopePublic rule.
func publicRule(resourceType string) ResourceRule {
	return ResourceRule{ResourceType: resourceType, Scope: ScopePublic}
}

// ref helpers for compact rule declarations.
func pRef(path string) Ref      { return Ref{Path: path, Target: constvars.ResourcePatient} }
func pracRef(path string) Ref   { return Ref{Path: path, Target: constvars.ResourcePractitioner} }
func roleRef(path string) Ref   { return Ref{Path: path, Target: constvars.ResourcePractitionerRole} }
func personRef(path string) Ref { return Ref{Path: path, Target: constvars.ResourcePerson} }
func idRef(target string) Ref   { return Ref{Path: "id", Target: target} }
