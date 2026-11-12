package federation

import (
	"context"
	"crypto/ecdh"
	"net/http"

	"github.com/konsulin-care/gateway-sdk-go/envelope"
)

// PepperListingName is the free federation-pepper delivery listing name
// (the pepperListingName constant of pepper_fetch.go), exported for the
// registerFederation wiring to bind the writer handler under.
const PepperListingName = pepperListingName

// EnvPepper exposes the APP_PEPPER manual-delivery fallback (envPepper of
// pepper.go) as the Pepper seam of NewPepperDelivery: it errors exactly
// when APP_PEPPER is unset or empty, which the handler answers 503 for.
//
// Parameters: none; reads APP_PEPPER from the environment.
// Returns: the normalized pepper bytes and nil, or an error when APP_PEPPER
// is unset. Never panics; never logs the pepper.
func EnvPepper() ([]byte, error) {
	return envPepper()
}

// PepperDeliveryDeps carries the injectable seams of the federation-pepper
// delivery handler: reading the caller party off the verified request,
// resolving that party's use:enc key, and loading the plaintext pepper.
// Production wires GetSub to gateway.ClaimsFromContext, ResolveKey to the
// SDK EncKeyResolver over the party JWKS and Pepper to envPepper; tests
// inject fakes.
type PepperDeliveryDeps struct {
	// GetSub returns the caller party id (the verified sub claim).
	GetSub func(*http.Request) string
	// ResolveKey returns the party's use:enc public key and its kid, or an
	// error when the party has no usable use:enc key.
	ResolveKey func(context.Context, string) (*ecdh.PublicKey, string, error)
	// Pepper returns the plaintext federation pepper, or an error when none
	// is available. The pepper is never logged.
	Pepper func() ([]byte, error)
}

// NewPepperDelivery builds the HTTP handler that serves one federation
// pepper delivery (BK-PEPPER-002): it reads the caller party from the
// verified claims, resolves that party's use:enc key — a resolution
// failure answers 403 fail-closed (no pepper is ever sealed to an unusable
// key) — loads the pepper — a load failure answers 503 — and seals the
// pepper with the plain ECDH-ES envelope to the caller's key, answering
// 200 with Content-Type application/jose and the compact JWE as the body.
// The pepper, the key material and the envelope are never logged.
//
// Parameters: d carries the GetSub, ResolveKey and Pepper seams (must be
// non-nil; a nil seam fails closed with the matching status).
// Returns: func(http.ResponseWriter, *http.Request) serving the delivery.
// Never panics.
func NewPepperDelivery(d PepperDeliveryDeps) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if d.GetSub == nil || d.ResolveKey == nil || d.Pepper == nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		ctx := r.Context()
		sub := d.GetSub(r)
		key, kid, err := d.ResolveKey(ctx, sub)
		if err != nil || key == nil {
			http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
			return
		}
		pepper, err := d.Pepper()
		if err != nil || len(pepper) == 0 {
			http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
			return
		}
		compact, err := envelope.Seal(pepper, kid, key)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/jose")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(compact))
	}
}
