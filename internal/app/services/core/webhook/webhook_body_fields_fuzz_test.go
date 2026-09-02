package webhook

import (
	"encoding/json"
	"testing"
)

// FuzzParseJSONBodyFields exercises parseJSONBodyFields, the contact-field
// extraction entry point for incoming JWT-payload webhooks (magic-link
// delivery). Webhook bodies are external input: valid JSON must always parse
// successfully (the usecase only surfaces WEBHOOK_* errors on malformed
// JSON), and no input may panic the extractor.
func FuzzParseJSONBodyFields(f *testing.F) {
	seeds := [][]byte{
		[]byte(`{"email":"patient@example.com","phone_number":"081234567890","chatwoot_id":"conv-1"}`),
		[]byte(`{"email":"practitioner@example.com"}`),
		[]byte(`{"phone_number":"0812","chatwoot_id":"w1"}`),
		[]byte(`{}`),
		[]byte(`null`),
		[]byte(`not json`),
	}
	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		_, _, _, err := parseJSONBodyFields(data)
		if json.Valid(data) && err != nil {
			t.Fatalf("valid JSON rejected by parseJSONBodyFields: %v", err)
		}
	})
}
