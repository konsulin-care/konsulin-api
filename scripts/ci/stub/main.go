// Package main implements a unified CI stub server for the Konsulin
// integration test environment. It replaces both the Node.js magiclink-stub
// and provides a Xendit API mock, serving all external service dependencies
// from a single lightweight Go binary.
//
// Routes:
//
//	Magiclink:
//	  POST /magiclink/send-magiclink                — app webhook forwarder target
//	  GET  /magiclink/api/v2/domains/public/inboxes/:org  — mailbox listing
//	  GET  /magiclink/api/v2/domains/public/messages/:id/links — magic-link URLs
//
//	Xendit:
//	  POST /v2/invoices         — create invoice (returns PENDING)
//	  GET  /v2/invoices/:id     — get invoice (returns PAID for re-verification)
//	  POST /v2/invoices/:id/expire — expire invoice
//
//	Health:
//	  GET /health — readiness probe
//
// Usage:
//
//	ci-stub [-addr :8081] [-healthcheck]
//
// The -healthcheck flag runs a single GET /health check and exits. This is
// used by Docker Compose HEALTHCHECK to verify the container is alive.
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
)

func main() {
	addr := flag.String("addr", ":8081", "listen address")
	healthcheck := flag.Bool("healthcheck", false, "run health check and exit")
	flag.Parse()

	if *healthcheck {
		resp, err := http.Get("http://localhost" + *addr + "/health")
		if err != nil {
			os.Exit(1)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			os.Exit(1)
		}
		os.Exit(0)
	}

	mux := newRouter()
	log.Printf("ci-stub listening on %s", *addr)
	// Codacy false-positive: CI-internal stub, TLS not applicable.
	if err := http.ListenAndServe(*addr, mux); err != nil {
		log.Fatalf("ci-stub: %v", err)
	}
}

// newRouter creates an http.ServeMux with all stub routes registered.
// Extracted for testability.
func newRouter() *http.ServeMux {
	mux := http.NewServeMux()

	// Health
	mux.HandleFunc("GET /health", handleHealth)

	// Magiclink routes — all under /magiclink/
	mux.HandleFunc("POST /magiclink/send-magiclink", handleSendMagiclink)
	mux.HandleFunc("GET /magiclink/api/v2/domains/public/inboxes/{org}", handleInboxListing)
	mux.HandleFunc("GET /magiclink/api/v2/domains/public/messages/{id}/links", handleMessageLinks)

	// Xendit routes
	mux.HandleFunc("POST /v2/invoices", handleCreateInvoice)
	mux.HandleFunc("GET /v2/invoices/{id}", handleGetInvoice)
	mux.HandleFunc("POST /v2/invoices/{id}/expire", handleExpireInvoice)

	return mux
}

func handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprint(w, "ok")
}
