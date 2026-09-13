package ownership

import (
	"bufio"
	"os"
	"strings"
	"testing"

	"konsulin-service/internal/pkg/constvars"

	"github.com/stretchr/testify/assert"
)

// TestRefPathConstants pins the FHIR reference-path constants used by the
// Rules table. The compartment conformance tests resolve these paths against
// the vendored R4 CompartmentDefinitions, so a wrong value here fails those
// tests too; this test documents the exact strings.
func TestRefPathConstants(t *testing.T) {
	cases := map[string]string{
		refSubject:   "subject.reference",
		refPatient:   "patient.reference",
		refActor:     "participant.#.actor.reference",
		refSender:    "sender.reference",
		refRecipient: "recipient.reference",
		refPerformer: "performer.#.reference",
		refAuthor:    "author.reference",
		refEnterer:   "enterer.reference",
		refProvider:  "provider.reference",
	}
	for constant, want := range cases {
		assert.Equal(t, want, constant, "constant value")
	}
	assert.Len(t, cases, 9, "all flagged duplicate literals are constantized")
}

// rbacPolicyRow is one parsed rbac_policy.csv row.
type rbacPolicyRow struct {
	role   string
	method string
	path   string
}

// rbacPolicyRows parses resources/rbac_policy.csv into structured rows,
// skipping comments and malformed lines.
func rbacPolicyRows(t *testing.T) []rbacPolicyRow {
	t.Helper()
	var rows []rbacPolicyRow
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
		rows = append(rows, rbacPolicyRow{
			role:   strings.TrimSpace(parts[1]),
			method: strings.TrimSpace(parts[2]),
			path:   strings.TrimSpace(parts[3]),
		})
	}
	assert.NoError(t, sc.Err())
	return rows
}

// reachableResourceTypes returns the set of FHIR resource types reachable
// through any /fhir/ policy path.
func reachableResourceTypes(t *testing.T) map[string]struct{} {
	t.Helper()
	types := map[string]struct{}{}
	for _, row := range rbacPolicyRows(t) {
		if !strings.HasPrefix(row.path, "/fhir/") {
			continue
		}
		seg := strings.TrimPrefix(row.path, "/fhir/")
		resourceType := strings.SplitN(seg, "/", 2)[0]
		if resourceType != "" {
			types[resourceType] = struct{}{}
		}
	}
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

// TestRulesCoverMutatingPolicyRows is the method x path completeness gate: any
// DELETE/PATCH policy row for a FHIR identity role (the roles that flow through
// the ownership engine) must target a type whose rule can scope a mutating
// query — an identity type (Patient/Practitioner, scoped by owned path id or
// _id) or a rule carrying at least one SearchParam. A future row such as
// "p, Patient, DELETE, /fhir/Media" fails CI until the engine can scope it.
// Guest and Superadmin rows are exempt: those roles bypass the ownership
// engine entirely (no FHIR identity to scope).
func TestRulesCoverMutatingPolicyRows(t *testing.T) {
	identityRoles := map[string]struct{}{
		constvars.KonsulinRolePatient:      {},
		constvars.KonsulinRolePractitioner: {},
		constvars.KonsulinRoleResearcher:   {},
		constvars.KonsulinRoleClinicAdmin:  {},
	}
	for _, row := range rbacPolicyRows(t) {
		if row.method != "DELETE" && row.method != "PATCH" {
			continue
		}
		if _, ok := identityRoles[row.role]; !ok {
			continue
		}
		if !strings.HasPrefix(row.path, "/fhir/") {
			continue
		}
		resourceType := strings.SplitN(strings.TrimPrefix(row.path, "/fhir/"), "/", 2)[0]
		rule, ok := Rules[resourceType]
		assert.True(t, ok, "DELETE/PATCH row %q %q targets type %q with no ownership rule", row.role, row.path, resourceType)
		if !ok {
			continue
		}
		isIdentity := resourceType == constvars.ResourcePatient || resourceType == constvars.ResourcePractitioner
		scopable := rule.Scope != ScopeInternal && len(rule.SearchParams) > 0
		assert.True(t, isIdentity || scopable,
			"DELETE/PATCH row %q %q targets %q which cannot scope a mutating query (identity type or ≥1 SearchParam required)",
			row.role, row.path, resourceType)
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
