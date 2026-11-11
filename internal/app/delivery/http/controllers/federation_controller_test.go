package controllers

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"konsulin-service/internal/app/drivers/gateway"

	"github.com/alicebob/miniredis/v2"
	"github.com/konsulin-care/gateway-sdk-go/gatewaytest"
	"github.com/konsulin-care/gateway-sdk-go/wire"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// eventTestEnv wires one fake gateway JWKS endpoint, one miniredis and the
// environment gateway.VerifyMiddleware reads, and returns a minter plus the
// built ks-event+jwt middleware — the same pipeline the federation router
// mounts in front of FederationController.HandleEvent. Servers and env vars
// restore automatically on cleanup.
//
// Parameters: t is the test harness.
// Returns: the token minter and the built middleware. Fails on error.
func newEventTestEnv(t *testing.T) (*gatewaytest.Minter, func(http.Handler) http.Handler) {
	t.Helper()
	minter, err := gatewaytest.NewMinter()
	require.NoError(t, err)
	jwks, err := minter.JWKS()
	require.NoError(t, err)
	srv, err := gatewaytest.NewJWKSServer(jwks)
	require.NoError(t, err)
	t.Cleanup(srv.Close)

	mr := miniredis.RunT(t)
	host, port, err := net.SplitHostPort(mr.Addr())
	require.NoError(t, err)
	t.Setenv("REDIS_HOST", host)
	t.Setenv("REDIS_PORT", port)
	t.Setenv("REDIS_PASSWORD", "")
	t.Setenv("APP_GATEWAY_URL", srv.URL())
	t.Setenv("APP_PARTY_ID", "pty-test-self")

	mw, err := gateway.VerifyMiddleware(wire.TypEvent)
	require.NoError(t, err)
	return minter, mw
}

// mintEvent signs a well-formed ks-event+jwt claims object (gateway iss,
// this party as aud, one of the four event purposes, a webhook id and the
// body_sha256 digest over body) with the fake gateway key.
//
// Parameters: t is the test harness; minter signs; purpose is the event
// purpose claim; webhookID is the webhook_id claim; body is the exact
// request body bytes the digest covers.
// Returns: the compact JWS. Fails on any minting error.
func mintEvent(t *testing.T, minter *gatewaytest.Minter, purpose, webhookID string, body []byte) string {
	t.Helper()
	now := time.Now()
	sum := sha256.Sum256(body)
	claims := map[string]any{
		"iss":         "gateway",
		"aud":         "pty-test-self",
		"sub":         "pty-test-sender",
		"purpose":     purpose,
		"webhook_id":  webhookID,
		"body_sha256": base64.RawURLEncoding.EncodeToString(sum[:]),
		"iat":         now.Add(-time.Minute).Unix(),
		"exp":         now.Add(5 * time.Minute).Unix(),
	}
	payload, err := json.Marshal(claims)
	require.NoError(t, err)
	tok, err := minter.Mint(wire.TypEvent, payload)
	require.NoError(t, err)
	return tok
}

// newEventRequest builds one inbound federation callback request carrying
// the bearer token and the exact body bytes.
//
// Parameters: tok is the gateway-signed event token; body is the payload.
// Returns: the request ready to serve.
func newEventRequest(tok string, body []byte) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/api/v1/federation/events", bytes.NewReader(body))
	r.Header.Set("Authorization", "Bearer "+tok)
	return r
}

// TestHandleEventDispatchesRegisteredPurpose covers assertion 1: an event
// whose purpose has a registered handler is dispatched to exactly that
// handler and answered 200.
//
// Parameters: t is the test harness.
// Returns: nothing; fails on any deviation.
func TestHandleEventDispatchesRegisteredPurpose(t *testing.T) {
	minter, mw := newEventTestEnv(t)
	ctrl := NewFederationController(zap.NewNop())

	var dispatched int
	var seenPurpose string
	ctrl.RegisterEventHandler(PurposeServiceResult, func(r *http.Request) error {
		dispatched++
		seenPurpose = gateway.ClaimsFromContext(r.Context()).Purpose
		return nil
	})

	body := []byte(`{"event":"result.ready"}`)
	h := mw(http.HandlerFunc(ctrl.HandleEvent))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, newEventRequest(mintEvent(t, minter, PurposeServiceResult, "wh-dispatch-1", body), body))

	assert.Equal(t, http.StatusOK, rec.Code,
		"a registered purpose must be answered 200")
	assert.Equal(t, 1, dispatched,
		"the event must be dispatched to the handler registered for its purpose")
	assert.Equal(t, PurposeServiceResult, seenPurpose,
		"the handler must see the verified purpose claim")
}

// TestHandleEventUnknownPurposeAcknowledgedAndHandlerErrorFailed covers
// assertion 2: a purpose with no registered handler is acknowledged 200
// without dispatch, and a handler that returns an error is answered 500 so
// the gateway retries.
//
// Parameters: t is the test harness.
// Returns: nothing; fails on any deviation.
func TestHandleEventUnknownPurposeAcknowledgedAndHandlerErrorFailed(t *testing.T) {
	minter, mw := newEventTestEnv(t)
	ctrl := NewFederationController(zap.NewNop())

	var dispatched int
	ctrl.RegisterEventHandler(PurposeServiceResult, func(_ *http.Request) error {
		dispatched++
		return errors.New("federation test: handler failure")
	})
	h := mw(http.HandlerFunc(ctrl.HandleEvent))

	t.Run("unknown purpose acknowledged 200 without dispatch", func(t *testing.T) {
		body := []byte(`{"event":"payment.paid"}`)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, newEventRequest(
			mintEvent(t, minter, PurposeSettlementNotification, "wh-unknown-1", body), body))

		assert.Equal(t, http.StatusOK, rec.Code,
			"an unknown purpose must be acknowledged 200")
		assert.Zero(t, dispatched,
			"an unknown purpose must not dispatch to any handler")
	})

	t.Run("handler error answered 500", func(t *testing.T) {
		body := []byte(`{"event":"result.ready"}`)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, newEventRequest(
			mintEvent(t, minter, PurposeServiceResult, "wh-error-1", body), body))

		assert.Equal(t, http.StatusInternalServerError, rec.Code,
			"a handler error must be answered 500 so the gateway retries")
		assert.Equal(t, 1, dispatched,
			"a registered purpose must dispatch exactly once")
	})
}
