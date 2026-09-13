package controllers

import (
	"encoding/json"
	"testing"
)

// FuzzValidateJSONBody exercises validateJSONBody, the JSON acceptance gate for
// the async enqueue webhook route (POST /{prefix}/hook/{service}). Webhook
// bodies are external input: valid JSON (object, null, scalar, array) must
// always pass the gate so the request is forwarded as-is, and only malformed
// JSON may be rejected. Mirrors FuzzParseJSONBodyFields to keep the sync and
// async gates from drifting apart.
func FuzzValidateJSONBody(f *testing.F) {
	seeds := [][]byte{
		[]byte(`{"email":"patient@example.com","phone_number":"081234567890","chatwoot_id":"conv-1"}`),
		[]byte(`{"email":"practitioner@example.com"}`),
		[]byte(`{"phone_number":"0812","chatwoot_id":"w1"}`),
		[]byte(`{}`),
		[]byte(`null`),
		[]byte(`not json`),
		[]byte(`0`), // regression seed: FuzzParseJSONBodyFields/582528ddfad69eb5
		[]byte(`"scalar"`),
		[]byte(`[1,2,3]`),
	}
	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		err := validateJSONBody(data)
		if json.Valid(data) && err != nil {
			t.Fatalf("valid JSON rejected by validateJSONBody: %v", err)
		}
	})
}
