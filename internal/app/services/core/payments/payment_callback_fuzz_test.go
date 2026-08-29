package payments

import (
	"encoding/json"
	"testing"

	"konsulin-service/internal/pkg/dto/requests"
)

// FuzzParseXenditInvoiceCallback exercises the Xendit invoice webhook callback
// body decoding entry point (webhook controller → paymentUsecase.
// XenditInvoiceCallback). Xendit callbacks are external input, so a malformed
// payload must never panic and an accepted body must always re-serialize — the
// reconciled amount/external_id drive payment state transitions downstream.
func FuzzParseXenditInvoiceCallback(f *testing.F) {
	seeds := [][]byte{
		[]byte(`{"id":"inv_123","external_id":"payment:trx_abc","status":"PAID","amount":15000.0,"currency":"IDR","created":"2026-01-01T00:00:00Z"}`),
		[]byte(`{"id":"inv_456","external_id":"payment:trx_def","status":"EXPIRED"}`),
		[]byte(`{"id":"inv_789","external_id":"payment:trx_ghi","status":"PENDING","amount":0}`),
		[]byte(`{"external_id":"payment:trx_jkl","status":"SETTLED"}`),
		[]byte(`{}`),
		[]byte(`{"id":"inv","external_id":"","status":"UNKNOWN"}`),
		[]byte(`not json`),
	}
	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		var body requests.XenditInvoiceCallbackBody
		if err := json.Unmarshal(data, &body); err != nil {
			return // malformed input is rejected by the decoder
		}
		if _, err := json.Marshal(body); err != nil {
			t.Fatalf("decoder accepted a callback body that cannot be re-marshaled: %v", err)
		}
	})
}
