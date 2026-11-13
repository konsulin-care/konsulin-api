package research

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// pseudonymKeyA is the fixed test HMAC key (test-only material).
const pseudonymKeyA = "research-test-key-a"

// pseudonymKeyB is a second key proving key sensitivity of the pseudonym.
const pseudonymKeyB = "research-test-key-b"

// patientResource returns a Patient populated with every direct identifier
// field, a full birthDate and clinical fields that must survive.
func patientResource() map[string]any {
	return map[string]any{
		"resourceType": "Patient",
		"id":           "pat-1",
		"name":         []any{map[string]any{"family": "Lovelace", "given": []any{"Ada"}}},
		"telecom":      []any{map[string]any{"system": "email", "value": "ada@example.com"}},
		"address":      []any{map[string]any{"line": []any{"1 Main St"}, "city": "Bandung"}},
		"identifier":   []any{map[string]any{"system": "urn:mrn", "value": "MRN-1"}},
		"photo":        []any{map[string]any{"contentType": "image/png", "data": "aGVsbG8="}},
		"birthDate":    "1990-05-17",
		"gender":       "female",
		"active":       true,
	}
}

// TestDeidentify_StripsDirectIdentifiersAndReducesBirthDate asserts the
// first contract line: name, telecom, address, identifier and photo are
// removed, birthDate collapses to its year, and clinical fields survive
// while the input map stays untouched.
func TestDeidentify_StripsDirectIdentifiersAndReducesBirthDate(t *testing.T) {
	in := patientResource()

	out, err := Deidentify(in, pseudonymKeyA)
	require.NoError(t, err)

	for _, field := range []string{"name", "telecom", "address", "identifier", "photo"} {
		assert.NotContains(t, out, field, "direct identifier %q must be removed", field)
	}
	assert.Equal(t, "1990", out["birthDate"], "birthDate must be reduced to the year")

	// Non-identifier fields survive.
	assert.Equal(t, "Patient", out["resourceType"])
	assert.Equal(t, "pat-1", out["id"])
	assert.Equal(t, "female", out["gender"])
	assert.Equal(t, true, out["active"])

	// The input must never be mutated.
	require.Contains(t, in, "name", "Deidentify must operate on a copy")
	assert.Equal(t, "1990-05-17", in["birthDate"])
}

// TestDeidentify_PseudonymizesSubjectReferences asserts the second contract
// line: the subject reference is replaced by an HMAC pseudonym that is
// deterministic for the same key, different under another key, carries no
// trace of the original id, and drops the identifying display.
func TestDeidentify_PseudonymizesSubjectReferences(t *testing.T) {
	in := map[string]any{
		"resourceType": "Observation",
		"id":           "obs-1",
		"status":       "final",
		"subject": map[string]any{
			"reference": "Patient/pat-1",
			"display":   "Ada Lovelace",
		},
	}
	// Nested subject (e.g. a Bundle entry resource) must be handled too.
	bundle := map[string]any{
		"resourceType": "Bundle",
		"type":         "searchset",
		"entry": []any{
			map[string]any{"resource": in},
		},
	}

	first, err := Deidentify(in, pseudonymKeyA)
	require.NoError(t, err)
	second, err := Deidentify(in, pseudonymKeyA)
	require.NoError(t, err)
	otherKey, err := Deidentify(in, pseudonymKeyB)
	require.NoError(t, err)

	refOf := func(m map[string]any) string {
		sub, ok := m["subject"].(map[string]any)
		require.True(t, ok, "subject must remain a reference object")
		ref, ok := sub["reference"].(string)
		require.True(t, ok, "subject reference must be a string")
		return ref
	}

	ref1 := refOf(first)
	ref2 := refOf(second)
	refOther := refOf(otherKey)

	require.NotEqual(t, "Patient/pat-1", ref1, "subject reference must be pseudonymized")
	assert.Equal(t, ref1, ref2, "the same key must produce the same pseudonym")
	assert.NotEqual(t, ref1, refOther, "a different key must produce a different pseudonym")
	assert.Len(t, ref1, 64, "pseudonym must be a hex SHA-256 digest")
	assert.True(t, strings.ContainsAny(ref1, "0123456789abcdef"), "pseudonym must be hex")
	assert.NotContains(t, ref1, "pat-1", "pseudonym must not leak the original id")
	assert.NotContains(t, first["subject"], "display", "display may carry a patient name")

	// Nested subjects are pseudonymized with the same key determinism.
	outBundle, err := Deidentify(bundle, pseudonymKeyA)
	require.NoError(t, err)
	entry := outBundle["entry"].([]any)[0].(map[string]any)
	nested := entry["resource"].(map[string]any)
	assert.Equal(t, ref1, refOf(nested), "nested subject must share the deterministic pseudonym")

	// Original subject untouched on the input.
	assert.Equal(t, "Patient/pat-1", in["subject"].(map[string]any)["reference"])
}

// TestDeidentify_EmptyKeyFailsClosed asserts that a missing
// APP_RESEARCH_PSEUDONYM_KEY is an error rather than an unpeppered hash.
func TestDeidentify_EmptyKeyFailsClosed(t *testing.T) {
	out, err := Deidentify(patientResource(), "  ")
	require.ErrorIs(t, err, ErrEmptyPseudonymKey)
	assert.Nil(t, out)
}
