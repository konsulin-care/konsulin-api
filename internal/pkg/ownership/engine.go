package ownership

import (
	"encoding/json"
	"errors"
	"fmt"
	"konsulin-service/internal/pkg/constvars"
	"net/url"
	"strings"

	"github.com/tidwall/gjson"
)

// ErrUnclassified is returned by write/search validation for resource types
// that have no ownership rule: the fail-closed flip denies unknown types.
var ErrUnclassified = errors.New("unclassified resource type has no ownership rule")

// Rule returns the ownership rule for a resource type and whether one exists.
func Rule(resourceType string) (ResourceRule, bool) {
	rule, ok := Rules[resourceType]
	return rule, ok
}

// OwnedBy decides whether the caller (oc) owns the given FHIR resource.
// Public resources are always owned; Internal and unclassified resources are
// denied (fail-closed flip). Ownership is proven by matching the rule's Refs
// against the caller's identities, by a code allowance (researcher/clinic
// admin), or by a per-type Checker.
func OwnedBy(raw []byte, resourceType string, oc *OwnershipContext) (bool, error) {
	rule, ok := Rules[resourceType]
	if !ok {
		return false, nil
	}
	if rule.Scope == ScopePublic {
		return true, nil
	}
	if rule.Scope == ScopeInternal {
		return false, nil
	}
	if !gjson.ValidBytes(raw) {
		return false, errors.New("invalid resource JSON")
	}
	if len(rule.PractitionerRoleCodings) > 0 && !holdsAnyCoding(oc, rule.PractitionerRoleCodings) {
		return false, nil
	}
	return matchRuleRefs(raw, rule, oc)
}

// matchRuleRefs resolves a rule's Refs, code allowances, and Checker strategy
// against the caller's identities.
func matchRuleRefs(raw []byte, rule ResourceRule, oc *OwnershipContext) (bool, error) {
	for _, ref := range rule.Refs {
		matched, err := refMatches(raw, ref, oc)
		if err != nil {
			return false, err
		}
		if matched {
			return true, nil
		}
	}
	if len(rule.CodeAllow) > 0 && holdsAnyCoding(oc, rule.CodeAllow) {
		return true, nil
	}
	if rule.Checker != nil {
		ok, err := rule.Checker(raw, oc)
		if err != nil {
			return false, err
		}
		if ok {
			return true, nil
		}
	}
	return false, nil
}

// ValidateWriteBody validates a POST/PUT body against the rule's WriteRefs.
// strict=true (PUT) requires each WriteRef to be present and match the caller;
// strict=false (POST) tolerates a missing ref. Callers holding a
// WriteBypassCodes coding are exempt.
func ValidateWriteBody(raw []byte, resourceType string, oc *OwnershipContext, strict bool) error {
	rule, ok := Rules[resourceType]
	if !ok {
		return fmt.Errorf("%w: %q", ErrUnclassified, resourceType)
	}
	if rule.Scope == ScopeInternal {
		return fmt.Errorf("internal resource type %q is not writable via the FHIR proxy", resourceType)
	}
	// Superadmin and code-exempt callers (e.g. clinic admin managing
	// PractitionerRole resources for others) are not constrained by WriteRefs.
	if oc.HasSuperadminRole {
		return nil
	}
	if len(rule.WriteBypassCodes) > 0 && holdsAnyCoding(oc, rule.WriteBypassCodes) {
		return nil
	}
	for _, ref := range rule.WriteRefs {
		matched, err := writeRefMatches(raw, ref, oc, strict)
		if err != nil {
			return err
		}
		if !matched {
			return fmt.Errorf("write ownership cannot be proven: %s must reference an owned %s", ref.Path, ref.Target)
		}
	}
	return nil
}

// ValidSearchQuery validates an entry-level search or a non-GET (DELETE/PATCH)
// request against the rule's scoping rules. It mirrors the legacy
// allowScopedEntryRead / ownsPatientQuery / ownsPractitionerQuery semantics:
//   - single-resource reads (no query) are exempt, except identity resources
//     whose path id must be owned (DELETE /Patient/{id});
//   - aggregate _summary=count queries stay public;
//   - QuestionnaireResponse/Communication entry-level reads require an
//     identity scope (patient own-refs, researcher code allowances,
//     practitioner/superadmin exemptions);
//   - public rules with SearchParams deny when a present param references a
//     non-owned identity.
func ValidSearchQuery(rawURL, resourceType string, oc *OwnershipContext) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	rule, ok := Rules[resourceType]
	if !ok {
		return false
	}

	// Identity resources: a path id must be owned (e.g. DELETE /Patient/{id});
	// collection deletes must scope via _id.
	if resourceType == constvars.ResourcePatient || resourceType == constvars.ResourcePractitioner {
		if id := pathResourceID(u.Path); id != "" {
			return ownedIDForTarget(resourceType, id, oc)
		}
		if u.RawQuery == "" {
			return false
		}
		return ownedIDForTarget(resourceType, u.Query().Get("_id"), oc)
	}

	// Single-resource reads (no query) are exempt; aggregate counts stay public.
	if u.RawQuery == "" {
		return true
	}
	if u.Query().Get("_summary") == "count" {
		return true
	}

	// Entry-level scoped reads (QuestionnaireResponse / Communication).
	if resourceType == constvars.ResourceQuestionnaireResponse || resourceType == constvars.ResourceCommunication {
		return validScopedEntrySearch(u, rule, resourceType, oc)
	}

	if rule.Scope == ScopePublic && len(rule.SearchParams) > 0 {
		return validPublicSearchQuery(u.Query(), rule, oc)
	}
	if rule.Scope == ScopeInternal {
		return false
	}
	return true
}

// validScopedEntrySearch implements the scoped-entry rules: a patient must
// scope via an owned SearchParam, researcher-coded callers must satisfy a
// SearchAllowance, then the legacy superadmin/practitioner exemptions apply.
func validScopedEntrySearch(u *url.URL, rule ResourceRule, resourceType string, oc *OwnershipContext) bool {
	q := u.Query()
	if oc.HasPatientRole && len(oc.PatientIDs) > 0 &&
		queryHasOwnRef(q, oc.PatientIDs, rule.SearchParams...) {
		return true
	}
	if held, anchored := searchAllowanceMatched(rule, oc, q); held {
		return anchored
	}
	if oc.HasSuperadminRole {
		// Superadmin is exempt for Communication; legacy QR behavior keeps the
		// identifier scope requirement.
		if resourceType == constvars.ResourceCommunication {
			return true
		}
		return q.Get("identifier") != ""
	}
	if oc.HasPractitionerRole {
		// Plain practitioners keep their existing authz; the response filter
		// (OwnedBy) is the gate for what they can actually read.
		return true
	}
	if resourceType == constvars.ResourceQuestionnaireResponse {
		return q.Get("identifier") != ""
	}
	return false
}

// searchAllowanceMatched reports whether the caller holds an allowance coding
// and, if so, whether any allowance param anchors the search.
func searchAllowanceMatched(rule ResourceRule, oc *OwnershipContext, q url.Values) (held, anchored bool) {
	if len(rule.SearchAllowances) == 0 || !holdsAnyCoding(oc, allowanceCodings(rule)) {
		return false, false
	}
	for _, a := range rule.SearchAllowances {
		if !holdsAnyCoding(oc, a.PractitionerRoleCodings) {
			continue
		}
		for _, p := range a.Params {
			if q.Get(p) != "" {
				return true, true
			}
		}
	}
	return true, false
}

// validPublicSearchQuery implements the legacy public-resource query scoping:
// when no ownership param is present the listing is public; when an ownership
// param (or a _has...practitioner param) is present, every value must
// reference an owned identity.
func validPublicSearchQuery(q url.Values, rule ResourceRule, oc *OwnershipContext) bool {
	if !hasOwnershipParam(q, rule) {
		return true
	}
	for _, param := range rule.SearchParams {
		for _, val := range q[param] {
			if val == "" {
				continue
			}
			for _, v := range strings.Split(val, ",") {
				if !refValueOwned(v, oc) {
					return false
				}
			}
		}
	}
	return validHasPractitionerParams(q, oc)
}

// hasOwnershipParam reports whether the query carries an ownership-scoping
// param (a rule SearchParam or a _has...practitioner reverse include).
func hasOwnershipParam(q url.Values, rule ResourceRule) bool {
	for key, vals := range q {
		if contains(rule.SearchParams, key) && len(vals) > 0 && vals[0] != "" {
			return true
		}
		if strings.HasPrefix(key, "_has") && strings.Contains(key, "practitioner") {
			return true
		}
	}
	return false
}

// validHasPractitionerParams validates _has...practitioner reverse-include
// params against the caller's owned ids.
func validHasPractitionerParams(q url.Values, oc *OwnershipContext) bool {
	for key, vals := range q {
		if strings.HasPrefix(key, "_has") && strings.Contains(key, "practitioner") {
			for _, v := range vals {
				if !ownedID(v, oc) {
					return false
				}
			}
		}
	}
	return true
}

// ShouldRedact reports whether a Communication response must be reduced to the
// rule's RedactKeep fields: researcher-coded or superadmin callers are
// redacted. The caller-side "scoped to own sender/recipient" exemption is
// handled by the middleware before this is consulted.
func ShouldRedact(resourceType string, oc *OwnershipContext) bool {
	if resourceType != constvars.ResourceCommunication {
		return false
	}
	rule, ok := Rules[resourceType]
	if !ok || len(rule.RedactKeep) == 0 {
		return false
	}
	return oc.HasSuperadminRole || oc.HoldsCoding(CodingResearcher)
}

// RedactKeepFields returns the fields kept when a resource type is redacted.
func RedactKeepFields(resourceType string) []string {
	rule, ok := Rules[resourceType]
	if !ok {
		return nil
	}
	return rule.RedactKeep
}

// refMatches resolves a Ref against the resource JSON and reports whether it
// references an owned identity of the ref's Target type.
func refMatches(raw []byte, ref Ref, oc *OwnershipContext) (bool, error) {
	if ref.Path == "id" {
		var env struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(raw, &env); err != nil {
			return false, err
		}
		return ownedIDForTarget(ref.Target, env.ID, oc), nil
	}
	result := gjson.GetBytes(raw, ref.Path)
	if !result.Exists() {
		return false, nil
	}
	if result.IsArray() {
		for _, item := range result.Array() {
			if item.Type == gjson.String && ownedRefValue(ref.Target, item.String(), oc) {
				return true, nil
			}
		}
		return false, nil
	}
	if result.Type == gjson.String {
		return ownedRefValue(ref.Target, result.String(), oc), nil
	}
	return false, nil
}

// writeRefMatches resolves a WriteRef and reports whether the body proves
// ownership. When the ref is absent, the result is !strict (POST lenient, PUT
// strict). When present, every value referencing the Target type must be
// owned, and at least one Target-typed value must exist.
func writeRefMatches(raw []byte, ref Ref, oc *OwnershipContext, strict bool) (bool, error) {
	result := gjson.GetBytes(raw, ref.Path)
	if !result.Exists() {
		return !strict, nil
	}
	// The "id" ref matches the resource's own bare id against the target's
	// owned ids (e.g. a Patient PUT must carry the caller's own id).
	if ref.Path == "id" {
		return ownedIDForTarget(ref.Target, result.String(), oc), nil
	}
	var values []string
	if result.IsArray() {
		for _, item := range result.Array() {
			if item.Type == gjson.String {
				values = append(values, item.String())
			}
		}
	} else if result.Type == gjson.String {
		values = append(values, result.String())
	}
	if len(values) == 0 {
		return !strict, nil
	}
	matched := false
	for _, v := range values {
		if !refHasTargetType(v, ref.Target) {
			continue
		}
		matched = true
		if !ownedRefValue(ref.Target, v, oc) {
			return false, nil
		}
	}
	return matched, nil
}

// ownedRefValue reports whether a FHIR reference value ("Patient/pat-1") of the
// given target type points at an identity the caller owns.
func ownedRefValue(target, value string, oc *OwnershipContext) bool {
	if !strings.HasPrefix(value, target+"/") {
		return false
	}
	id := strings.TrimPrefix(value, target+"/")
	return ownedIDForTarget(target, id, oc)
}

// ownedIDForTarget reports whether an id is owned under the target type.
func ownedIDForTarget(target, id string, oc *OwnershipContext) bool {
	switch target {
	case constvars.ResourcePatient:
		_, ok := oc.PatientIDs[id]
		return ok
	case constvars.ResourcePractitioner:
		_, ok := oc.PractitionerIDs[id]
		return ok
	case constvars.ResourcePractitionerRole:
		_, ok := oc.PractitionerRoleIDs[id]
		return ok
	case constvars.ResourcePerson:
		_, ok := oc.PersonIDs[id]
		return ok
	}
	return false
}

// refHasTargetType reports whether a reference value names the given target
// type (e.g. "Practitioner/prac-1" has type "Practitioner").
func refHasTargetType(value, target string) bool {
	return strings.HasPrefix(value, target+"/")
}

// ownedID reports whether a bare id is owned under any identity type.
func ownedID(id string, oc *OwnershipContext) bool {
	if id == "" {
		return false
	}
	_, p := oc.PatientIDs[id]
	_, pr := oc.PractitionerIDs[id]
	_, r := oc.PractitionerRoleIDs[id]
	_, pe := oc.PersonIDs[id]
	return p || pr || r || pe
}

// refValueOwned reports whether a reference value ("Patient/pat-1",
// "Practitioner/prac-1", or a bare id) references an owned identity.
func refValueOwned(value string, oc *OwnershipContext) bool {
	for _, target := range []string{
		constvars.ResourcePatient,
		constvars.ResourcePractitioner,
		constvars.ResourcePractitionerRole,
		constvars.ResourcePerson,
	} {
		if strings.HasPrefix(value, target+"/") {
			return ownedRefValue(target, value, oc)
		}
	}
	return ownedID(value, oc)
}

// queryHasOwnRef reports whether any of the given query params reference one of
// the owned ids, tolerating the "Patient/" prefix and comma-separated lists.
func queryHasOwnRef(q url.Values, ids map[string]struct{}, params ...string) bool {
	for _, key := range params {
		for _, val := range q[key] {
			for _, v := range strings.Split(val, ",") {
				id := strings.TrimPrefix(v, constvars.FHIRRefPrefixPatient)
				_, ok := ids[id]
				if ok {
					return true
				}
			}
		}
	}
	return false
}

// allowanceCodings collects every coding required by the rule's search
// allowances.
func allowanceCodings(rule ResourceRule) []string {
	var out []string
	for _, a := range rule.SearchAllowances {
		out = append(out, a.PractitionerRoleCodings...)
	}
	return out
}

// holdsAnyCoding reports whether the caller holds any of the given codings.
func holdsAnyCoding(oc *OwnershipContext, codings []string) bool {
	for _, c := range codings {
		if oc.HoldsCoding(c) {
			return true
		}
	}
	return false
}

// pathResourceID extracts "Patient/pat-1" style ids from a URL path, handling
// both "/fhir/Patient/pat-1" and "/Patient/pat-1" shapes.
func pathResourceID(path string) string {
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(parts) >= 2 {
		if strings.EqualFold(parts[0], "fhir") && len(parts) >= 3 {
			return parts[2]
		}
		return parts[1]
	}
	return ""
}

// contains reports whether s is in list.
func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
