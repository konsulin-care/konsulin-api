package bundle

import (
	"encoding/json"
	"testing"

	"konsulin-service/internal/pkg/fhir_dto"
)

// FuzzParseFHIRBundle exercises the FHIR bundle response decoding entry point
// used by BundleFhirClientImpl.PostTransactionBundle. Blaze responses are
// external input, so a malformed bundle must never panic and any bundle the
// decoder accepts must be re-serializable (the gateway re-marshals bundles
// when forwarding transaction responses).
func FuzzParseFHIRBundle(f *testing.F) {
	seeds := [][]byte{
		[]byte(`{"resourceType":"Bundle","type":"transaction-response","total":1,"link":[{"relation":"self","url":"http://localhost/fhir"}],"entry":[{"resource":{"resourceType":"OperationOutcome","issue":[]}}]}`),
		[]byte(`{"resourceType":"Bundle","type":"searchset","total":0,"entry":[]}`),
		[]byte(`{"resourceType":"Bundle","type":"batch-response"}`),
		[]byte(`{"resourceType":"Bundle","total":2,"entry":[{"resource":{}},{"resource":{"resourceType":"Patient","id":"p1"}}]}`),
		[]byte(`{}`),
		[]byte(`not json`),
	}
	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		var bundle fhir_dto.FHIRBundle
		if err := json.Unmarshal(data, &bundle); err != nil {
			return // malformed input is rejected by the decoder
		}
		if _, err := json.Marshal(bundle); err != nil {
			t.Fatalf("decoder accepted a bundle that cannot be re-marshaled: %v", err)
		}
	})
}
