package privacy

import (
	"konsulin-service/internal/pkg/constvars"
	"strings"

	"github.com/tidwall/gjson"
)

// maxReferenceScanDepth bounds the recursive reference walk so pathological
// nesting cannot overflow the stack (mirrors the depth cap in proxy.go).
const maxReferenceScanDepth = 30

// collectReferences appends every "reference" string found at any depth of a
// parsed JSON value, mirroring FHIR's Reference structure without needing the
// resource schema.
func collectReferences(v gjson.Result, out *[]string, depth int) {
	if v.Type != gjson.JSON || depth > maxReferenceScanDepth {
		return
	}
	v.ForEach(func(key, value gjson.Result) bool {
		if key.String() == "reference" && value.Type == gjson.String {
			*out = append(*out, value.String())
		}
		collectReferences(value, out, depth+1)
		return true
	})
}

// isDeletable reports whether a FHIR resource body may be fully deleted during
// a purge. A resource is deletable only if it carries no reference to another
// user: no Patient/{x} reference for a patient other than the one being purged
// and no Practitioner/{x} reference at all. Shared resources (co-authored
// observations, referral communications) are kept and left referencing the
// retained Patient shell. References to non-user resources (ResearchStudy,
// Consent, PractitionerRole, ...) do not block deletion.
func isDeletable(body []byte, patientID string) bool {
	patientSelf := constvars.FHIRRefPrefixPatient + patientID
	var refs []string
	collectReferences(gjson.ParseBytes(body), &refs, 0)
	for _, r := range refs {
		if strings.HasPrefix(r, constvars.FHIRRefPrefixPatient) && r != patientSelf {
			return false
		}
		if strings.HasPrefix(r, constvars.FHIRRefPrefixPractitioner) {
			return false
		}
	}
	return true
}
