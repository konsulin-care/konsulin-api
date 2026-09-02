package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// xenditStore holds in-memory state for stubbed Xendit invoices.
type xenditStore struct {
	mu       sync.RWMutex
	invoices map[string]*xenditInvoice
}

type xenditInvoice struct {
	ID         string `json:"id"`
	ExternalID string `json:"external_id"`
	Status     string `json:"status"`
	InvoiceURL string `json:"invoice_url"`
	Amount     int    `json:"amount"`
	Currency   string `json:"currency"`
}

var xendit = &xenditStore{
	invoices: make(map[string]*xenditInvoice),
}

// handleCreateInvoice accepts a Xendit-style invoice creation request,
// generates a synthetic ID, and returns a PENDING invoice.
func handleCreateInvoice(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		ExternalID string `json:"external_id"`
		Amount     int    `json:"amount"`
		Currency   string `json:"currency"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON"})
		return
	}

	if payload.Currency == "" {
		payload.Currency = "IDR"
	}

	inv := &xenditInvoice{
		ID:         fmt.Sprintf("inv_stub_%d", time.Now().UnixNano()),
		ExternalID: payload.ExternalID,
		Status:     "PENDING",
		InvoiceURL: fmt.Sprintf("http://localhost:8082/pay/approve/inv_stub_%d", time.Now().UnixNano()),
		Amount:     payload.Amount,
		Currency:   payload.Currency,
	}

	xendit.mu.Lock()
	xendit.invoices[inv.ID] = inv
	xendit.mu.Unlock()

	writeJSON(w, http.StatusOK, inv)
}

// handleGetInvoice returns an invoice with status PAID. This simulates
// Xendit's re-verification endpoint — after a callback arrives, the app
// calls GET to confirm the invoice is truly paid.
func handleGetInvoice(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	xendit.mu.RLock()
	inv, ok := xendit.invoices[id]
	xendit.mu.RUnlock()

	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	// Return a copy with PAID status (never modify the stored original)
	resp := *inv
	resp.Status = "PAID"
	writeJSON(w, http.StatusOK, resp)
}

// handleExpireInvoice sets an invoice's status to EXPIRED. This is called
// by Xendit when an invoice times out, and by Bruno tests to verify the
// callback for expired invoices.
func handleExpireInvoice(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	xendit.mu.Lock()
	inv, ok := xendit.invoices[id]
	if ok {
		inv.Status = "EXPIRED"
	}
	xendit.mu.Unlock()

	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	writeJSON(w, http.StatusOK, inv)
}
