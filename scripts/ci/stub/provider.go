package main

import (
	"crypto/ecdh"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/konsulin-care/gateway-sdk-go/envelope"
	"github.com/konsulin-care/gateway-sdk-go/verify"
	"github.com/konsulin-care/gateway-sdk-go/wire"
)

// Provider-party mode configuration (environment variables).
const (
	providerEnvPartyID  = "PROVIDER_PARTY_ID"
	providerEnvJWKSURL  = "PROVIDER_GATEWAY_JWKS_URL"
	providerEnvEncKey   = "PROVIDER_ENC_PRIVATE_KEY"
	providerEnvReplyKey = "PROVIDER_REPLY_ENC_PUBLIC_KEY"
)

// errorKey is the shared JSON field name of the stub's error responses.
const errorKey = "error"

// providerRecord is one opened delivery as served at GET /provider/last.
type providerRecord struct {
	Payload   string `json:"payload"`
	Service   string `json:"service"`
	WebhookID string `json:"webhook_id"`
}

// providerStore holds the last opened payload; last is nil until the first
// delivery opens. Safe for concurrent use.
type providerStore struct {
	mu   sync.RWMutex
	last *providerRecord
}

// set records the newest opened delivery.
// Parameters: rec is the record to store.
func (s *providerStore) set(rec *providerRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.last = rec
}

// get returns the last recorded delivery, or nil when nothing was opened.
func (s *providerStore) get() *providerRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.last
}

// reset clears the record; used by tests to start from an empty state.
func (s *providerStore) reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.last = nil
}

// providerLast is the process-wide record of the last opened delivery.
var providerLast = &providerStore{}

// providerConfig is the provider-party mode configuration: the stub's own
// party identity, the gateway JWKS endpoint it verifies deliveries against,
// its X25519 use:enc private key (opens request envelopes), and the caller's
// X25519 use:enc public key (seals sync replies back).
type providerConfig struct {
	partyID  string
	jwksURL  string
	encPriv  *ecdh.PrivateKey
	replyPub *ecdh.PublicKey
}

// providerConfigFromEnv reads the provider-party mode configuration from
// the environment, failing closed on any missing or malformed value.
// Returns: the config and nil, or nil and an error naming the bad variable.
func providerConfigFromEnv() (*providerConfig, error) {
	partyID := strings.TrimSpace(os.Getenv(providerEnvPartyID))
	if partyID == "" {
		return nil, fmt.Errorf("%s not set", providerEnvPartyID)
	}
	jwksURL := strings.TrimSpace(os.Getenv(providerEnvJWKSURL))
	if jwksURL == "" {
		return nil, fmt.Errorf("%s not set", providerEnvJWKSURL)
	}
	encRaw := strings.TrimSpace(os.Getenv(providerEnvEncKey))
	if encRaw == "" {
		return nil, fmt.Errorf("%s not set", providerEnvEncKey)
	}
	encPriv, err := envelope.ParseX25519PrivateKey(encRaw)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", providerEnvEncKey, err)
	}
	replyRaw := strings.TrimSpace(os.Getenv(providerEnvReplyKey))
	if replyRaw == "" {
		return nil, fmt.Errorf("%s not set", providerEnvReplyKey)
	}
	replyPub, err := parseX25519PublicKey(replyRaw)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", providerEnvReplyKey, err)
	}
	return &providerConfig{
		partyID:  partyID,
		jwksURL:  jwksURL,
		encPriv:  encPriv,
		replyPub: replyPub,
	}, nil
}

// parseX25519PublicKey parses standard (padded) base64 wrapping exactly 32
// bytes into an X25519 public key. Fail-closed on bad base64 or length.
// Parameters: s is the key value (std base64).
// Returns: the public key and nil, or an error. Never panics.
func parseX25519PublicKey(s string) (*ecdh.PublicKey, error) {
	raw, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("provider: invalid base64: %w", err)
	}
	if len(raw) != 32 {
		return nil, fmt.Errorf("provider: decoded length is %d bytes, want 32", len(raw))
	}
	key, err := ecdh.X25519().NewPublicKey(raw)
	if err != nil {
		return nil, fmt.Errorf("provider: X25519 public key: %w", err)
	}
	return key, nil
}

// providerHandler serves gateway-signed deliveries: verify happens in the
// SDK middleware, this handler opens the bound envelope, records the opened
// payload, and answers 200 (async: result ticket present) or a sealed reply
// (sync: no result ticket).
type providerHandler struct {
	cfg    *providerConfig
	nonces wire.ReplayCache
}

// registerProvider registers the provider-party routes on mux. Without a
// valid configuration the delivery route answers 503 and the stub keeps
// serving its other services.
// Parameters: mux is the router built by newRouter.
func registerProvider(mux *http.ServeMux) {
	disabled := func(err error) {
		log.Printf("ci-stub: provider-party mode disabled: %v", err)
		mux.HandleFunc("POST /provider/invoke", handleProviderUnavailable)
		mux.Handle("GET /provider/last", http.HandlerFunc(handleProviderLast))
	}
	cfg, err := providerConfigFromEnv()
	if err != nil {
		disabled(err)
		return
	}
	verifier, err := verify.NewVerifier(cfg.jwksURL, cfg.partyID)
	if err != nil {
		disabled(err)
		return
	}
	h := &providerHandler{cfg: cfg, nonces: verify.NewMemoryReplayCache(time.Now)}
	mux.Handle("POST /provider/invoke",
		verify.Middleware(verifier, wire.TypDelivery)(http.HandlerFunc(h.handleDelivery)))
	mux.Handle("GET /provider/last", http.HandlerFunc(handleProviderLast))
}

// handleDelivery opens the bound envelope carried by the delivery body and
// records the opened payload. The SDK middleware already verified the
// ks-delivery+jwt bearer and the webhook-id dedupe before this runs.
// A body over 1 MiB, a missing envelope, unusable claims, or a rejected
// envelope answer 400 without recording anything. An async delivery (a
// non-empty result_ticket) answers 200; a sync delivery answers with a
// bound-envelope reply sealed to the caller's use:enc key.
// Parameters: w writes the response; r carries verified claims and the body.
func (h *providerHandler) handleDelivery(w http.ResponseWriter, r *http.Request) {
	claims := verify.ClaimsFromContext(r.Context())
	var delivery wire.DeliveryBody
	if err := json.NewDecoder(r.Body).Decode(&delivery); err != nil || delivery.Envelope == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{errorKey: "invalid delivery body"})
		return
	}
	exp, err := envelope.ExpectationsFromClaims(claims)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{errorKey: "delivery claims unusable"})
		return
	}
	data, err := envelope.OpenBound(r.Context(), delivery.Envelope, h.cfg.encPriv, exp, h.nonces, time.Now)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{errorKey: "envelope rejected"})
		return
	}
	providerLast.set(&providerRecord{
		Payload:   string(data),
		Service:   claims.Service,
		WebhookID: claims.WebhookID,
	})
	if delivery.ResultTicket != "" {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
		return
	}
	h.writeSealedReply(w, claims)
}

// writeSealedReply answers a sync delivery with a bound envelope sealed to
// the caller's use:enc key (from/to swapped, same service) as the raw body
// with Content-Type application/jose.
// Parameters: w writes the reply; claims are the verified delivery claims.
func (h *providerHandler) writeSealedReply(w http.ResponseWriter, claims verify.Claims) {
	reply, err := json.Marshal(map[string]any{
		"status":     "ok",
		"service":    claims.Service,
		"webhook_id": claims.WebhookID,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{errorKey: "reply encode failed"})
		return
	}
	kid, err := envelope.EncJWK(h.cfg.replyPub)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{errorKey: "reply key unusable"})
		return
	}
	sealed, err := envelope.SealBound(reply, kid.Kid, h.cfg.replyPub, envelope.Binding{
		From:    h.cfg.partyID,
		To:      claims.Sub,
		Service: claims.Service,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{errorKey: "reply seal failed"})
		return
	}
	w.Header().Set("Content-Type", "application/jose")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(sealed))
}

// handleProviderLast serves the last opened payload at GET /provider/last;
// 404 when no delivery has been opened.
// Parameters: w writes the response; r is unused.
func handleProviderLast(w http.ResponseWriter, _ *http.Request) {
	rec := providerLast.get()
	if rec == nil {
		writeJSON(w, http.StatusNotFound, map[string]any{errorKey: "no delivery recorded"})
		return
	}
	writeJSON(w, http.StatusOK, rec)
}

// handleProviderUnavailable answers the delivery route when provider-party
// mode has no valid configuration.
// Parameters: w writes the response; r is unused.
func handleProviderUnavailable(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusServiceUnavailable, map[string]any{errorKey: "provider-party mode not configured"})
}
