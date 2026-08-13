package ownership

import "konsulin-service/internal/pkg/constvars"

// OwnershipContext describes the FHIR identities (Patient, Practitioner,
// PractitionerRole, Person) a caller owns for the current request, plus the
// practitioner-role codings (system|code) the caller holds. It is built once
// per request and reused by every ownership decision in the request.
type OwnershipContext struct {
	HasPatientRole      bool
	HasPractitionerRole bool
	HasSuperadminRole   bool
	PatientIDs          map[string]struct{}
	PractitionerIDs     map[string]struct{}
	PractitionerRoleIDs map[string]struct{}
	// PractitionerRoleCodings holds "system|code" pairs, e.g.
	// "http://terminology.hl7.org/CodeSystem/practitioner-role|researcher".
	PractitionerRoleCodings map[string]struct{}
	PersonIDs               map[string]struct{}
}

// NewContext returns an OwnershipContext with initialized maps.
func NewContext() *OwnershipContext {
	return &OwnershipContext{
		PatientIDs:              map[string]struct{}{},
		PractitionerIDs:         map[string]struct{}{},
		PractitionerRoleIDs:     map[string]struct{}{},
		PractitionerRoleCodings: map[string]struct{}{},
		PersonIDs:               map[string]struct{}{},
	}
}

// AddPatientID records a patient the caller owns.
func (oc *OwnershipContext) AddPatientID(id string) {
	if id != "" {
		oc.PatientIDs[id] = struct{}{}
	}
}

// AddPractitionerID records a practitioner the caller owns.
func (oc *OwnershipContext) AddPractitionerID(id string) {
	if id != "" {
		oc.PractitionerIDs[id] = struct{}{}
	}
}

// AddPractitionerRoleID records a practitioner role the caller owns.
func (oc *OwnershipContext) AddPractitionerRoleID(id string) {
	if id != "" {
		oc.PractitionerRoleIDs[id] = struct{}{}
	}
}

// AddPersonID records a Person resource the caller owns.
func (oc *OwnershipContext) AddPersonID(id string) {
	if id != "" {
		oc.PersonIDs[id] = struct{}{}
	}
}

// AddCoding records a practitioner-role coding the caller holds as a
// "system|code" string. Pass the Phase 1 constants, e.g.
// constvars.FhirPractitionerRoleSystemHL7 and
// constvars.FhirPractitionerRoleCodeResearcher.
func (oc *OwnershipContext) AddCoding(system, code string) {
	if system != "" && code != "" {
		oc.PractitionerRoleCodings[system+"|"+code] = struct{}{}
	}
}

// HoldsCoding reports whether the caller holds the given "system|code" coding.
func (oc *OwnershipContext) HoldsCoding(systemCode string) bool {
	_, ok := oc.PractitionerRoleCodings[systemCode]
	return ok
}

// Coding constants used by code-conditioned rules.
const (
	// CodingResearcher is the "system|code" pair for the Researcher role.
	CodingResearcher = constvars.FhirPractitionerRoleSystemHL7 + "|" + constvars.FhirPractitionerRoleCodeResearcher
	// CodingClinicAdmin is the "system|code" pair for the Clinic Admin role.
	CodingClinicAdmin = constvars.FhirPractitionerRoleSystemSnomed + "|" + constvars.FhirPractitionerRoleCodeAdministrativeStaff
)
