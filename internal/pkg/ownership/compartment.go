package ownership

import (
	"encoding/json"
	"fmt"
	"strings"
)

// CompartmentDefinition JSON shape (FHIR R4):
//
//	{
//	  "resourceType": "CompartmentDefinition",
//	  "code": "Patient",
//	  "resource": [
//	    { "code": "Observation", "param": ["subject", "performer"] },
//	    ...
//	  ]
//	}
type compartmentResource struct {
	Code  string   `json:"code"`
	Param []string `json:"param"`
}

type compartmentDefinition struct {
	ResourceType string                `json:"resourceType"`
	Code         string                `json:"code"`
	Resource     []compartmentResource `json:"resource"`
}

// compartmentDefinitionResourceType is the FHIR resourceType of a
// CompartmentDefinition document.
const compartmentDefinitionResourceType = "CompartmentDefinition"

// ParseCompartmentDefinition parses a vendored FHIR R4 CompartmentDefinition
// JSON document and returns resourceType -> compartment params. It validates
// that the document's compartment code matches wantCode.
func ParseCompartmentDefinition(data []byte, wantCode string) (map[string][]string, error) {
	var def compartmentDefinition
	if err := json.Unmarshal(data, &def); err != nil {
		return nil, err
	}
	if def.ResourceType != compartmentDefinitionResourceType {
		return nil, fmt.Errorf("not a CompartmentDefinition: %s", def.ResourceType)
	}
	if !strings.EqualFold(def.Code, wantCode) {
		return nil, fmt.Errorf("compartment code %q does not match expected %q", def.Code, wantCode)
	}
	out := make(map[string][]string, len(def.Resource))
	for _, r := range def.Resource {
		out[r.Code] = r.Param
	}
	return out, nil
}

// normalizeRefPath converts an ownership Ref.Path (a gjson path) into a FHIR
// compartment parameter (base element path): ".reference" suffixes and gjson
// array wildcards are stripped; "$this" (compartment param meaning "the
// resource itself") is handled by the caller. Examples:
//
//	"subject.reference"                 -> "subject"
//	"participant.#.actor.reference"     -> "participant.actor"
//	"performer.#.reference"             -> "performer"
func normalizeRefPath(path string) string {
	p := strings.ReplaceAll(path, ".#.", ".")
	p = strings.TrimSuffix(p, ".reference")
	return p
}
