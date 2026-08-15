package ownership

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

// loadCompartmentFixture loads and parses a vendored FHIR R4 compartment
// fixture. The fixtures live at resources/fhir/CompartmentDefinition-{code}.json
// and are fetched once from hl7.org at vendoring time.
func loadCompartmentFixture(t *testing.T, path, wantCode string) map[string][]string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read compartment fixture %s: %v", path, err)
	}
	compartment, err := ParseCompartmentDefinition(data, wantCode)
	if err != nil {
		t.Fatalf("failed to parse compartment fixture %s: %v", path, err)
	}
	return compartment
}

// compartmentParamOverrides maps ownership element paths to the compartment
// SEARCH PARAMETER they satisfy. The FHIR CompartmentDefinition lists search
// parameters, which for many resources resolve to a different element path
// (e.g. Condition's "patient" search param maps to element subject). The
// conformance test resolves each normalized ref path through this table before
// comparing against the compartment params, so a typo'd element path still
// fails the test while legitimate search-param mappings stay documented.
var compartmentParamOverrides = map[string]map[string]string{
	"Condition":                {"subject": "patient"},
	"Encounter":                {"subject": "patient", "participant.individual": "participant"},
	"CarePlan":                 {"subject": "patient", "performer.actor": "performer"},
	"Goal":                     {"subject": "patient"},
	"Procedure":                {"subject": "patient"},
	"Appointment":              {"participant.actor": "actor"},
	"Invoice":                  {"participant.actor": "participant"},
	"MedicationAdministration": {"performer.actor": "performer"},
}

// nonCompartmentRefs documents reference paths that intentionally have no home
// in the FHIR R4 patient/practitioner CompartmentDefinitions. Each entry is a
// justified exception: the path is a genuine FHIR reference element but the
// compartment spec does not list it as a compartment parameter.
var nonCompartmentRefs = map[string]map[string]string{
	// Task has no compartment params in R4; owner is the only practitioner
	// reference element on the resource.
	"Task": {"owner": "Task has no R4 compartment params; owner is the owning practitioner"},
	// Contract has no compartment params; author/authority are its only
	// practitioner-facing references.
	"Contract": {"author": "Contract has no R4 compartment params; author is the practitioner author"},
	// PaymentNotice has no patient-compartment params; provider/recipient are
	// the natural owner refs.
	"PaymentNotice": {"provider": "PaymentNotice has no R4 patient compartment param; provider is the owner"},
	// PaymentReconciliation is Internal (gateway-only); no refs needed.
	"PaymentReconciliation": {},
}

// assertRefsInCompartment asserts every ref of an allowed target type resolves
// to a compartment search parameter for resource type rt. Paths documented in
// allowed (nonCompartmentRefs) are exempt.
func assertRefsInCompartment(t *testing.T, rt string, refs []Ref, params []string, allowedTargets map[string]bool, compartmentCode string, allowed map[string]string) {
	t.Helper()
	for _, ref := range refs {
		if ref.Path == "id" || !allowedTargets[ref.Target] {
			continue
		}
		norm := resolveCompartmentParam(rt, ref.Path)
		if contains(params, norm) {
			continue
		}
		if _, ok := allowed[norm]; ok {
			continue
		}
		t.Errorf("rule %q ref %q (compartment param %q) not in %s compartment params %v", rt, ref.Path, norm, compartmentCode, params)
	}
}

// TestRefsConformToPatientCompartment asserts that every Patient-scoped ref in
// the Rules table exists as a parameter of the FHIR R4 Patient compartment.
// It is one-directional: the compartment may list more params than we use.
func TestRefsConformToPatientCompartment(t *testing.T) {
	compartment := loadCompartmentFixture(t, "../../../resources/fhir/CompartmentDefinition-patient.json", "Patient")
	for rt, rule := range Rules {
		if !ruleHasPatientRefs(rule) {
			continue
		}
		assertRefsInCompartment(t, rt, rule.Refs, compartment[rt], map[string]bool{"Patient": true}, "Patient", nonCompartmentRefs[rt])
	}
}

// TestRefsConformToPractitionerCompartment asserts that every Practitioner or
// PractitionerRole-scoped ref exists as a parameter of the FHIR R4
// Practitioner compartment.
func TestRefsConformToPractitionerCompartment(t *testing.T) {
	compartment := loadCompartmentFixture(t, "../../../resources/fhir/CompartmentDefinition-practitioner.json", "Practitioner")
	for rt, rule := range Rules {
		if !ruleHasPractitionerRefs(rule) {
			continue
		}
		assertRefsInCompartment(t, rt, rule.Refs, compartment[rt], map[string]bool{"Practitioner": true, "PractitionerRole": true}, "Practitioner", nonCompartmentRefs[rt])
	}
}

// resolveCompartmentParam normalizes a ref path and applies the per-resource
// search-param overrides.
func resolveCompartmentParam(resourceType, refPath string) string {
	norm := normalizeRefPath(refPath)
	if overrides, ok := compartmentParamOverrides[resourceType]; ok {
		if mapped, ok := overrides[norm]; ok {
			return mapped
		}
	}
	return norm
}

func TestNormalizeRefPath(t *testing.T) {
	cases := map[string]string{
		"subject.reference":                  "subject",
		"patient.reference":                  "patient",
		"participant.#.actor.reference":      "participant.actor",
		"performer.#.reference":              "performer",
		"participant.#.individual.reference": "participant.individual",
		"payor.#.reference":                  "payor",
		"actor.reference":                    "actor",
		"author.#.reference":                 "author",
		"individual.reference":               "individual",
		"id":                                 "id",
	}
	for in, want := range cases {
		assert.Equal(t, want, normalizeRefPath(in), "normalizeRefPath(%q)", in)
	}
}

func TestLoadCompartmentFixture(t *testing.T) {
	compartment := loadCompartmentFixture(t, "../../../resources/fhir/CompartmentDefinition-patient.json", "Patient")
	assert.NotEmpty(t, compartment)
	assert.Contains(t, compartment, "Observation")
	assert.Contains(t, compartment["Observation"], "subject")
}

// helpers shared with compartment.go tests

func ruleHasPatientRefs(rule ResourceRule) bool {
	for _, ref := range rule.Refs {
		if ref.Target == "Patient" {
			return true
		}
	}
	return false
}

func ruleHasPractitionerRefs(rule ResourceRule) bool {
	for _, ref := range rule.Refs {
		if ref.Target == "Practitioner" || ref.Target == "PractitionerRole" {
			return true
		}
	}
	return false
}
