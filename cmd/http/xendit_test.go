package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"konsulin-service/internal/app/config"

	"github.com/xendit/xendit-go/v7/invoice"
)

func TestNewXenditClient_DefaultEndpoint(t *testing.T) {
	for _, baseURL := range []string{"", "  "} {
		client := newXenditClient(config.AppXendit{APIKey: "synthetic-test-key", BaseURL: baseURL})
		got, err := client.GetConfig().ServerURL(0, nil)
		if err != nil || got != "https://api.xendit.co" {
			t.Fatalf("default endpoint = %q, %v", got, err)
		}
	}
}

func TestNewXenditClient_InvoiceUsesConfiguredEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v2/invoices/" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		user, _, ok := r.BasicAuth()
		if !ok || user != "synthetic-test-key" {
			t.Error("SDK did not preserve API key authentication")
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "inv_test", "external_id": "appointment:test", "status": "PENDING",
			"invoice_url": "http://stub/pay", "amount": 50000, "currency": "IDR",
		})
	}))
	defer server.Close()
	client := newXenditClient(config.AppXendit{APIKey: "synthetic-test-key", BaseURL: " " + server.URL + "/ "})
	request := invoice.CreateInvoiceRequest{ExternalId: "appointment:test", Amount: 50000}
	got, _, err := client.InvoiceApi.CreateInvoice(context.Background()).CreateInvoiceRequest(request).Execute()
	if err != nil {
		t.Fatalf("create invoice through stub: %v", err)
	}
	if got == nil || got.Id == nil || *got.Id != "inv_test" {
		t.Fatalf("unexpected invoice response: %+v", got)
	}
}
