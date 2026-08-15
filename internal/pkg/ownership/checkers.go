package ownership

import (
	"encoding/json"
	"konsulin-service/internal/pkg/constvars"
	"strings"
)

// invoiceChecker reproduces the legacy Invoice read rule: an Invoice is public
// only when ALL of its references point at Practitioner, PractitionerRole, or
// Device actors. Any reference to another type (e.g. a Patient) means the
// invoice is not public and ownership must be proven via the rule's Refs.
var invoiceChecker OwnershipChecker = func(raw []byte, _ *OwnershipContext) (bool, error) {
	whitelisted := map[string]struct{}{
		constvars.ResourcePractitioner:     {},
		constvars.ResourcePractitionerRole: {},
		constvars.ResourceDevice:           {},
	}

	var res map[string]any
	if err := json.Unmarshal(raw, &res); err != nil {
		return false, err
	}

	var refs []string
	collectRefs(res, &refs, 0)
	if len(refs) == 0 {
		return false, nil
	}
	for _, ref := range refs {
		parts := strings.SplitN(ref, "/", 2)
		if len(parts) == 0 {
			return false, nil
		}
		if _, ok := whitelisted[parts[0]]; !ok {
			return false, nil
		}
	}
	return true, nil
}

// collectRefs walks arbitrary JSON and collects all "reference" string values.
func collectRefs(m map[string]any, out *[]string, depth int) {
	if depth > 30 {
		return
	}
	for k, vv := range m {
		if k == "reference" {
			if s, ok := vv.(string); ok {
				*out = append(*out, s)
			}
			continue
		}
		collectRefValue(vv, out, depth+1)
	}
}

// collectRefValue walks a nested JSON value, recursing into objects and arrays.
func collectRefValue(v any, out *[]string, depth int) {
	switch t := v.(type) {
	case map[string]any:
		collectRefs(t, out, depth)
	case []any:
		for _, item := range t {
			collectRefValue(item, out, depth+1)
		}
	}
}
