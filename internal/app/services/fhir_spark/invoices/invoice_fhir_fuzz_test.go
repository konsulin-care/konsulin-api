package invoices

import (
	"encoding/json"
	"testing"

	"konsulin-service/internal/pkg/fhir_dto"
)

// invoiceSearchResponse mirrors the anonymous response struct decoded in
// invoiceFhirClient.Search: a FHIR searchset whose entries carry Invoice
// resources. Blaze JSON is external input, so malformed search responses must
// never panic and accepted responses must re-serialize cleanly (Invoice DTOs
// flow into payment reconciliation).
func FuzzParseInvoiceSearchResponse(f *testing.F) {
	seeds := [][]byte{
		[]byte(`{"resourceType":"Bundle","total":1,"entry":[{"fullUrl":"http://localhost/fhir/Invoice/i1","resource":{"resourceType":"Invoice","id":"i1","status":"paid","totalGross":{"value":15000,"currency":"IDR"}}}]}`),
		[]byte(`{"resourceType":"Bundle","total":0,"entry":[]}`),
		[]byte(`{"total":1,"entry":[{"resource":{}}]}`),
		[]byte(`{"resourceType":"Bundle","total":1,"entry":[{"resource":{"resourceType":"Invoice","lineItem":[{"sequence":1,"priceComponent":[{"type":"base","factor":1.5,"amount":{"value":10000}}]}]}}]}`),
		[]byte(`{}`),
		[]byte(`{"entry":null}`),
	}
	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		var result struct {
			Total        int    `json:"total"`
			ResourceType string `json:"resourceType"`
			Entry        []struct {
				FullUrl  string           `json:"fullUrl"`
				Resource fhir_dto.Invoice `json:"resource"`
			} `json:"entry"`
		}
		if err := json.Unmarshal(data, &result); err != nil {
			return // malformed input is rejected by the decoder
		}
		for _, entry := range result.Entry {
			if _, err := json.Marshal(entry.Resource); err != nil {
				t.Fatalf("decoder accepted an invoice that cannot be re-marshaled: %v", err)
			}
		}
	})
}
