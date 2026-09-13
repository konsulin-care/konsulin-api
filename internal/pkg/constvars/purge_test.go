package constvars

import (
	"reflect"
	"testing"
)

// TestPurgeRules_Registry pins the exact set of actively-owned resource types
// and their proof-of-ownership search parameters. Adding or removing a rule
// here is a deliberate purge-policy change and must be reviewed.
func TestPurgeRules_Registry(t *testing.T) {
	got := make(map[string][]string, len(PurgeRules))
	for _, rule := range PurgeRules {
		got[rule.ResourceType] = rule.Params
	}
	want := map[string][]string{
		ResourceQuestionnaireResponse: {"author"},
		ResourceCommunication:         {"sender"},
		ResourceConsent:               {"patient"},
		ResourceResearchSubject:       {"patient"},
		ResourceObservation:           {"subject"},
		ResourceCondition:             {"subject"},
		ResourceAppointment:           {"actor"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("PurgeRules = %v, want %v", got, want)
	}
}

// TestPurgeRules_SharedTypesNeverEnumerated guards the integration guarantee
// that shared/cross-user types are absent from the purge path by construction.
func TestPurgeRules_SharedTypesNeverEnumerated(t *testing.T) {
	for _, rule := range PurgeRules {
		switch rule.ResourceType {
		case ResourceResearchStudy, ResourcePlanDefinition, ResourceQuestionnaire, ResourceInvoice:
			t.Errorf("shared type %q must never be purged", rule.ResourceType)
		}
	}
}
