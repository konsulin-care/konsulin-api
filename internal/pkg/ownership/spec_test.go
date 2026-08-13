package ownership

import (
	"bufio"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// reachableResourceTypes parses resources/rbac_policy.csv and returns the set
// of FHIR resource types reachable through any policy path.
func reachableResourceTypes(t *testing.T) map[string]struct{} {
	t.Helper()
	types := map[string]struct{}{}
	f, err := os.Open("../../../resources/rbac_policy.csv")
	assert.NoError(t, err)
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Split(line, ",")
		if len(parts) < 4 {
			continue
		}
		path := strings.TrimSpace(parts[3])
		if !strings.HasPrefix(path, "/fhir/") {
			continue
		}
		seg := strings.TrimPrefix(path, "/fhir/")
		resourceType := strings.SplitN(seg, "/", 2)[0]
		if resourceType != "" {
			types[resourceType] = struct{}{}
		}
	}
	assert.NoError(t, sc.Err())
	return types
}

// TestRulesCoverReachableResourceTypes is the completeness gate: every resource
// type reachable through rbac_policy.csv paths must have an ownership rule, so
// a new policy row can never silently bypass ownership (fail-closed flip).
func TestRulesCoverReachableResourceTypes(t *testing.T) {
	for rt := range reachableResourceTypes(t) {
		_, ok := Rules[rt]
		assert.True(t, ok, "resource type %q reachable via rbac_policy.csv has no ownership rule", rt)
	}
}

// TestRulesCoverLegacyOwnershipSurface guards against silently dropping types
// that the previous ownership maps classified (patient/practitioner/public).
func TestRulesCoverLegacyOwnershipSurface(t *testing.T) {
	legacy := map[string]struct{}{
		// patient-scoped legacy set
		"Encounter": {}, "AllergyIntolerance": {}, "MedicationRequest": {},
		"Procedure": {}, "DiagnosticReport": {}, "ImagingStudy": {},
		"DocumentReference": {}, "CarePlan": {}, "Goal": {}, "RiskAssessment": {},
		"FamilyMemberHistory": {}, "Immunization": {}, "MedicationAdministration": {},
		"MedicationDispense": {}, "MedicationStatement": {}, "Coverage": {},
		"Claim": {}, "ExplanationOfBenefit": {}, "PaymentNotice": {},
		"PaymentReconciliation": {}, "Account": {}, "ChargeItem": {},
		// practitioner-scoped legacy set
		"CommunicationRequest": {}, "Task": {}, "Contract": {},
		"CoverageEligibilityRequest": {}, "CoverageEligibilityResponse": {},
		"ClaimResponse": {},
		// public legacy set
		"CodeSystem": {}, "ValueSet": {}, "ConceptMap": {}, "StructureDefinition": {},
		"OperationDefinition": {}, "SearchParameter": {}, "CompartmentDefinition": {},
		"GraphDefinition": {}, "ImplementationGuide": {}, "CapabilityStatement": {},
		"MessageDefinition": {}, "ActivityDefinition": {}, "Library": {},
		"Measure": {}, "MeasureReport": {}, "TestScript": {}, "TestReport": {},
		"Subscription": {}, "SubscriptionTopic": {}, "VerificationResult": {},
		"Requirements": {}, "ExampleScenario": {}, "SpecimenDefinition": {},
		"NamingSystem": {}, "TerminologyCapabilities": {},
	}
	for rt := range legacy {
		_, ok := Rules[rt]
		assert.True(t, ok, "legacy-classified resource type %q lost its ownership rule", rt)
	}
}

func TestRuleInvariants(t *testing.T) {
	for rt, rule := range Rules {
		assert.Equal(t, rt, rule.ResourceType, "rule key must match ResourceType")
		if rule.Scope == ScopePublic {
			assert.Empty(t, rule.Refs, "public rule %q must not carry ownership refs", rt)
			assert.Empty(t, rule.PractitionerRoleCodings, "public rule %q must not carry coding conditions", rt)
		}
		if rule.Scope == ScopeInternal {
			assert.Empty(t, rule.Refs, "internal rule %q must not carry ownership refs", rt)
		}
		for _, ref := range rule.Refs {
			assert.NotEmpty(t, ref.Path, "rule %q has empty ref path", rt)
			assert.Contains(t, []string{
				"Patient", "Practitioner", "PractitionerRole", "Person",
			}, ref.Target, "rule %q ref target invalid", rt)
		}
	}
}
