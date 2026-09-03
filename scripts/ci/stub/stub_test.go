package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func setupTestServer() *http.ServeMux {
	return newRouter()
}

// ── Health ───────────────────────────────────────────────────────────────

func TestHealthReturns200(t *testing.T) {
	mux := setupTestServer()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

// ── Magiclink: send-magiclink ────────────────────────────────────────────

func TestSendMagiclinkStoresLink(t *testing.T) {
	mux := setupTestServer()

	body, _ := json.Marshal(map[string]any{
		"url":   "https://app.example.com/verify?token=abc123",
		"exp":   60,
		"email": "user@org1.example.com",
	})
	req := httptest.NewRequest(http.MethodPost, "/magiclink/send-magiclink", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}
	if resp["success"] != true {
		t.Fatalf("expected success=true, got %v", resp["success"])
	}
}

func TestSendMagiclinkBadJSON(t *testing.T) {
	mux := setupTestServer()

	req := httptest.NewRequest(http.MethodPost, "/magiclink/send-magiclink", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// ── Magiclink: inbox listing ─────────────────────────────────────────────

func TestInboxListingReturnsMsgs(t *testing.T) {
	mux := setupTestServer()

	// First, send a magic link to seed the inbox
	body, _ := json.Marshal(map[string]any{
		"url":   "https://app.example.com/verify?token=test123",
		"exp":   60,
		"email": "inbox-test@myorg.example.com",
	})
	sendReq := httptest.NewRequest(http.MethodPost, "/magiclink/send-magiclink", bytes.NewReader(body))
	sendReq.Header.Set("Content-Type", "application/json")
	sendW := httptest.NewRecorder()
	mux.ServeHTTP(sendW, sendReq)

	if sendW.Code != http.StatusOK {
		t.Fatalf("send failed: %d", sendW.Code)
	}

	// Then, query the inbox
	req := httptest.NewRequest(http.MethodGet, "/magiclink/api/v2/domains/public/inboxes/myorg", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp struct {
		Msgs []struct {
			ID         string `json:"id"`
			SecondsAgo int    `json:"seconds_ago"`
		} `json:"msgs"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(resp.Msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(resp.Msgs))
	}
	if resp.Msgs[0].ID == "" {
		t.Fatal("message ID should not be empty")
	}
}

func TestInboxListingEmpty(t *testing.T) {
	mux := setupTestServer()

	req := httptest.NewRequest(http.MethodGet, "/magiclink/api/v2/domains/public/inboxes/nonexistent", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp struct {
		Msgs []any `json:"msgs"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(resp.Msgs) != 0 {
		t.Fatalf("expected 0 messages, got %d", len(resp.Msgs))
	}
}

// ── Magiclink: message links ─────────────────────────────────────────────

func TestMessageLinksReturnsURLs(t *testing.T) {
	mux := setupTestServer()

	// Send a magic link to get a message ID
	body, _ := json.Marshal(map[string]any{
		"url":   "https://app.example.com/verify?token=linktest",
		"exp":   60,
		"email": "links-test@linkorg.example.com",
	})
	sendReq := httptest.NewRequest(http.MethodPost, "/magiclink/send-magiclink", bytes.NewReader(body))
	sendReq.Header.Set("Content-Type", "application/json")
	sendW := httptest.NewRecorder()
	mux.ServeHTTP(sendW, sendReq)

	// Get the inbox to find the message ID
	inboxReq := httptest.NewRequest(http.MethodGet, "/magiclink/api/v2/domains/public/inboxes/linkorg", nil)
	inboxW := httptest.NewRecorder()
	mux.ServeHTTP(inboxW, inboxReq)

	var inboxResp struct {
		Msgs []struct {
			ID string `json:"id"`
		} `json:"msgs"`
	}
	json.Unmarshal(inboxW.Body.Bytes(), &inboxResp)
	msgID := inboxResp.Msgs[0].ID

	// Query links for that message
	req := httptest.NewRequest(http.MethodGet, "/magiclink/api/v2/domains/public/messages/"+msgID+"/links", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp struct {
		Links []string `json:"links"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(resp.Links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(resp.Links))
	}
	if resp.Links[0] != "https://app.example.com/verify?token=linktest" {
		t.Fatalf("unexpected link: %s", resp.Links[0])
	}
}

func TestMessageLinksNotFound(t *testing.T) {
	mux := setupTestServer()

	req := httptest.NewRequest(http.MethodGet, "/magiclink/api/v2/domains/public/messages/nonexistent/links", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp struct {
		Links []string `json:"links"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp.Links) != 0 {
		t.Fatalf("expected 0 links, got %d", len(resp.Links))
	}
}

// ── Xendit: create invoice ───────────────────────────────────────────────

func TestCreateInvoiceReturnsPending(t *testing.T) {
	mux := setupTestServer()

	body, _ := json.Marshal(map[string]any{
		"external_id": "order_123",
		"amount":      50000,
		"currency":    "IDR",
	})
	req := httptest.NewRequest(http.MethodPost, "/v2/invoices", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		ID         string `json:"id"`
		ExternalID string `json:"external_id"`
		Status     string `json:"status"`
		InvoiceURL string `json:"invoice_url"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if resp.ID == "" {
		t.Fatal("invoice ID should not be empty")
	}
	if resp.ExternalID != "order_123" {
		t.Fatalf("expected external_id=order_123, got %s", resp.ExternalID)
	}
	if resp.Status != statusPending {
		t.Fatalf("expected status=PENDING, got %s", resp.Status)
	}
	if resp.InvoiceURL == "" {
		t.Fatal("invoice_url should not be empty")
	}
}

// ── Xendit: get invoice (re-verification) ────────────────────────────────

func TestGetInvoiceReturnsPaid(t *testing.T) {
	mux := setupTestServer()

	// Create an invoice first
	body, _ := json.Marshal(map[string]any{
		"external_id": "verify_order",
		"amount":      50000,
	})
	createReq := httptest.NewRequest(http.MethodPost, "/v2/invoices", bytes.NewReader(body))
	createReq.Header.Set("Content-Type", "application/json")
	createW := httptest.NewRecorder()
	mux.ServeHTTP(createW, createReq)

	var createResp struct {
		ID string `json:"id"`
	}
	json.Unmarshal(createW.Body.Bytes(), &createResp)

	// GET the invoice — should return PAID (re-verification)
	req := httptest.NewRequest(http.MethodGet, "/v2/invoices/"+createResp.ID, nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Status != statusPaid {
		t.Fatalf("expected status=PAID, got %s", resp.Status)
	}
}

func TestGetInvoiceNotFound(t *testing.T) {
	mux := setupTestServer()

	req := httptest.NewRequest(http.MethodGet, "/v2/invoices/inv_nonexistent", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

// ── Xendit: expire invoice ───────────────────────────────────────────────

func TestExpireInvoiceReturnsExpired(t *testing.T) {
	mux := setupTestServer()

	// Create an invoice
	body, _ := json.Marshal(map[string]any{
		"external_id": "expire_order",
		"amount":      50000,
	})
	createReq := httptest.NewRequest(http.MethodPost, "/v2/invoices", bytes.NewReader(body))
	createReq.Header.Set("Content-Type", "application/json")
	createW := httptest.NewRecorder()
	mux.ServeHTTP(createW, createReq)

	var createResp struct {
		ID string `json:"id"`
	}
	json.Unmarshal(createW.Body.Bytes(), &createResp)

	// Expire it
	req := httptest.NewRequest(http.MethodPost, "/v2/invoices/"+createResp.ID+"/expire", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Status != statusExpired {
		t.Fatalf("expected status=EXPIRED, got %s", resp.Status)
	}
}

// ── Unknown routes return 404 ────────────────────────────────────────────

func TestUnknownRouteReturns404(t *testing.T) {
	mux := setupTestServer()

	req := httptest.NewRequest(http.MethodGet, "/unknown/path", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}
