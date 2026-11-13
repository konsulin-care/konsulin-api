// Package research implements the consent-gated research databank
// behaviors (BK-DATABANK): nightly extraction, de-identification,
// withdrawal and sealed export of derived FHIR resources.
package research

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
)

// directIdentifierFields are the FHIR fields carrying direct identifiers;
// they are removed from every resource at any nesting depth.
var directIdentifierFields = []string{"name", "telecom", "address", "identifier", "photo"}

// maxDeidentifyDepth bounds the recursive scrub walk (the ownership engine
// uses the same limit), so a pathological resource cannot blow the stack.
const maxDeidentifyDepth = 32

// ErrEmptyPseudonymKey reports that de-identification was attempted without
// an APP_RESEARCH_PSEUDONYM_KEY. The databank fails closed: a missing key
// disables extraction (config.DatabankEnabled), and a direct call must not
// produce an unpeppered — therefore reversible — pseudonym.
var ErrEmptyPseudonymKey = errors.New("research: deidentify: empty pseudonym key")

// Deidentify returns a de-identified copy of a FHIR resource for the
// research databank (BK-DATABANK-002): the direct identifier fields
// (name, telecom, address, identifier, photo) are stripped at any depth,
// birthDate is reduced to its year, and every subject reference is replaced
// with an HMAC-SHA256 pseudonym keyed by APP_RESEARCH_PSEUDONYM_KEY. The
// input map is never mutated.
//
// Parameters:
//   - resource is the FHIR resource (decoded JSON) to de-identify; a nil
//     map is returned as an empty result.
//   - pseudonymKey is the APP_RESEARCH_PSEUDONYM_KEY HMAC key.
//
// Returns the de-identified copy and nil, or nil and ErrEmptyPseudonymKey
// when the key is empty or whitespace. The pseudonym is deterministic for
// the same key and input, and changes with the key. Never panics.
func Deidentify(resource map[string]any, pseudonymKey string) (map[string]any, error) {
	if strings.TrimSpace(pseudonymKey) == "" {
		return nil, ErrEmptyPseudonymKey
	}
	if resource == nil {
		return map[string]any{}, nil
	}
	scrubbed, ok := scrubValue(resource, pseudonymKey, 0).(map[string]any)
	if !ok {
		return map[string]any{}, nil
	}
	return scrubbed, nil
}

// scrubValue recursively rebuilds v without the direct identifier fields,
// with birthDate reduced to its year, and with subject references replaced
// by keyed pseudonyms. Returns the scrubbed copy of v.
func scrubValue(v any, pseudonymKey string, depth int) any {
	if depth > maxDeidentifyDepth {
		return v
	}
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, vv := range t {
			switch {
			case containsField(directIdentifierFields, k):
				continue // direct identifier: dropped
			case k == "birthDate":
				if year, ok := birthYear(vv); ok {
					out[k] = year
				} // malformed birthDate: dropped (fail closed)
			case k == "subject":
				out[k] = scrubSubject(vv, pseudonymKey)
			default:
				out[k] = scrubValue(vv, pseudonymKey, depth+1)
			}
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, vv := range t {
			out[i] = scrubValue(vv, pseudonymKey, depth+1)
		}
		return out
	default:
		return t
	}
}

// scrubSubject replaces a FHIR Reference value with its keyed pseudonym.
// A string reference is pseudonymized outright; an object keeps only the
// pseudonymized "reference" and the harmless "type" — "display" and any
// other member are dropped because display commonly carries a patient name.
// Returns the pseudonymized subject value.
func scrubSubject(v any, pseudonymKey string) any {
	switch t := v.(type) {
	case string:
		return pseudonymize(t, pseudonymKey)
	case map[string]any:
		out := make(map[string]any, len(t))
		if ref, ok := t["reference"].(string); ok && ref != "" {
			out["reference"] = pseudonymize(ref, pseudonymKey)
		}
		if typ, ok := t["type"].(string); ok && typ != "" {
			out["type"] = typ
		}
		return out
	default:
		return v
	}
}

// pseudonymize returns the hex-encoded HMAC-SHA256 of value under key:
// deterministic for the same key and value, different across keys.
// Returns the hex digest.
func pseudonymize(value, key string) string {
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(value))
	return hex.EncodeToString(mac.Sum(nil))
}

// birthYear reduces a FHIR date value to its year. Only a string of at
// least four leading digits qualifies ("1990-05-17" -> "1990", "1990" ->
// "1990"); anything else reports false so the caller drops the field.
// Returns the year and true, or false.
func birthYear(v any) (string, bool) {
	s, ok := v.(string)
	if !ok || len(s) < 4 {
		return "", false
	}
	year := s[:4]
	for _, c := range year {
		if c < '0' || c > '9' {
			return "", false
		}
	}
	return year, true
}

// containsField reports whether field is one of fields.
// Returns true on a match.
func containsField(fields []string, field string) bool {
	for _, f := range fields {
		if f == field {
			return true
		}
	}
	return false
}
