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
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
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
	srv := &http.Server{
		Addr:              *addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       30 * time.Second,
	}

	go func() {
		log.Printf("ci-stub listening on %s", *addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("ci-stub: %v", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	sig := <-quit
	log.Printf("received %s, shutting down ci-stub", sig)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("ci-stub shutdown: %v", err)
		return
	}
	log.Println("ci-stub stopped")
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
