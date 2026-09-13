package fhir_dto

import (
	"bytes"
	"encoding/json"
	"testing"
)

// TestPractitionerRoleCodeRoundTrip verifies PractitionerRole.Code serializes
// to the FHIR code element as a list of CodeableConcept codings.
func TestPractitionerRoleCodeRoundTrip(t *testing.T) {
	role := PractitionerRole{
		ResourceType: "PractitionerRole",
		Active:       true,
		Practitioner: Reference{Reference: "Practitioner/pr-1"},
		Code: []CodeableConcept{{
			Coding: []Coding{{
				System:  "http://terminology.hl7.org/CodeSystem/practitioner-role",
				Code:    "researcher",
				Display: "Researcher",
			}},
		}},
	}

	b, err := json.Marshal(role)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	for _, want := range []string{`"researcher"`, `"Researcher"`, `"http://terminology.hl7.org/CodeSystem/practitioner-role"`, `"code"`} {
		if !bytes.Contains(b, []byte(want)) {
			t.Errorf("marshaled PractitionerRole missing %s: %s", want, string(b))
		}
	}

	var decoded PractitionerRole
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(decoded.Code) != 1 || len(decoded.Code[0].Coding) != 1 {
		t.Fatalf("decoded Code = %+v, want one CodeableConcept with one coding", decoded.Code)
	}
	coding := decoded.Code[0].Coding[0]
	if coding.System != "http://terminology.hl7.org/CodeSystem/practitioner-role" || coding.Code != "researcher" || coding.Display != "Researcher" {
		t.Errorf("decoded coding = %+v, want researcher coding", coding)
	}
}
