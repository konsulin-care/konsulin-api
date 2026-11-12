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
	"github.com/go-chi/chi/v5"
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
	return newCallbackTestEnv(t, wire.TypEvent)
}

// newDeliveryTestEnv wires the ks-delivery+jwt pipeline the federation router
// mounts in front of FederationController.HandleDelivery — the same fake
// gateway JWKS, miniredis and environment, verified as a delivery token.
//
// Parameters: t is the test harness.
// Returns: the token minter and the built delivery middleware. Fails on error.
func newDeliveryTestEnv(t *testing.T) (*gatewaytest.Minter, func(http.Handler) http.Handler) {
	t.Helper()
	return newCallbackTestEnv(t, wire.TypDelivery)
}

// newCallbackTestEnv wires one fake gateway JWKS endpoint, one miniredis and
// the environment gateway.VerifyMiddleware reads, and returns a minter plus
// the built middleware for kind — the same pipeline the federation router
// mounts in front of the federation controller.
//
// Parameters: t is the test harness; kind is the token typ to verify
// (wire.TypEvent or wire.TypDelivery).
// Returns: the token minter and the built middleware. Fails on error.
func newCallbackTestEnv(t *testing.T, kind string) (*gatewaytest.Minter, func(http.Handler) http.Handler) {
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

	mw, err := gateway.VerifyMiddleware(kind)
	require.NoError(t, err)
	return minter, mw
}

// mintDelivery signs a well-formed ks-delivery+jwt claims object (gateway
// iss, this party as aud, the webhook-delivery purpose, a webhook id, the
// body_sha256 digest over body and the conditional service/listing_id
// claims) with the fake gateway key.
//
// Parameters: t is the test harness; minter signs; service is the service
// claim (the listing's service); webhookID is the webhook_id claim; body is
// the exact request body bytes the digest covers.
// Returns: the compact JWS. Fails on any minting error.
func mintDelivery(t *testing.T, minter *gatewaytest.Minter, service, webhookID string, body []byte) string {
	t.Helper()
	now := time.Now()
	sum := sha256.Sum256(body)
	claims := map[string]any{
		"iss":         "gateway",
		"aud":         "pty-test-self",
		"sub":         "pty-test-sender",
		"purpose":     wire.PurposeWebhookDelivery,
		"webhook_id":  webhookID,
		"body_sha256": base64.RawURLEncoding.EncodeToString(sum[:]),
		"service":     service,
		"listing_id":  "lst-" + webhookID,
		"iat":         now.Add(-time.Minute).Unix(),
		"exp":         now.Add(5 * time.Minute).Unix(),
	}
	payload, err := json.Marshal(claims)
	require.NoError(t, err)
	tok, err := minter.Mint(wire.TypDelivery, payload)
	require.NoError(t, err)
	return tok
}

// newDeliveryRoute mounts HandleDelivery on a chi router exactly as the
// federation router does — POST /federation/deliveries/{listing} behind the
// verification middleware — so the {listing} path segment resolves.
//
// Parameters: mw is the built verification middleware; ctrl is the
// federation controller under test.
// Returns: the handler chain ready to serve.
func newDeliveryRoute(mw func(http.Handler) http.Handler, ctrl *FederationController) http.Handler {
	mux := chi.NewRouter()
	mux.With(mw).Post("/api/v1/federation/deliveries/{listing}", ctrl.HandleDelivery)
	return mux
}

// newDeliveryRequest builds one inbound federation delivery request carrying
// the bearer token, the exact body bytes and the listing path segment.
//
// Parameters: tok is the gateway-signed delivery token; body is the payload;
// listing is the {listing} path segment.
// Returns: the request ready to serve.
func newDeliveryRequest(tok string, body []byte, listing string) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/api/v1/federation/deliveries/"+listing, bytes.NewReader(body))
	r.Header.Set("Authorization", "Bearer "+tok)
	return r
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

// TestHandleDeliveryServiceClaimMismatchRejected covers assertion 1: a
// delivery whose verified service claim differs from the {listing} path
// segment is rejected 401 and never reaches the registered handler.
//
// Parameters: t is the test harness.
// Returns: nothing; fails on any deviation.
func TestHandleDeliveryServiceClaimMismatchRejected(t *testing.T) {
	minter, mw := newDeliveryTestEnv(t)
	ctrl := NewFederationController(zap.NewNop())

	var dispatched int
	ctrl.RegisterDeliveryHandler("svc-known", func(_ *http.Request) error {
		dispatched++
		return nil
	})

	body := []byte(`{"event":"delivery.ping"}`)
	h := newDeliveryRoute(mw, ctrl)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, newDeliveryRequest(
		mintDelivery(t, minter, "svc-other", "wh-delivery-1", body), body, "svc-known"))

	assert.Equal(t, http.StatusUnauthorized, rec.Code,
		"a delivery whose service claim differs from the path listing must be rejected 401")
	assert.Zero(t, dispatched,
		"a mismatched delivery must never reach the registered handler")
}

// TestHandleDeliveryWriterHandlerStatusPassesThrough covers the writer
// registration seam: a registered writer handler owns its response (its
// 403 passes through with no controller 200 on top), and the verified
// service-claim check still wins before the writer branch (a mismatched
// claim answers 401 and the writer never runs).
//
// Parameters: t is the test harness.
// Returns: nothing; fails on any deviation.
func TestHandleDeliveryWriterHandlerStatusPassesThrough(t *testing.T) {
	minter, mw := newDeliveryTestEnv(t)
	ctrl := NewFederationController(zap.NewNop())

	var writerCalls int
	ctrl.RegisterDeliveryWriterHandler("svc-pepper", func(w http.ResponseWriter, _ *http.Request) {
		writerCalls++
		w.WriteHeader(http.StatusForbidden)
	})

	body := []byte(`{"event":"delivery.ping"}`)
	h := newDeliveryRoute(mw, ctrl)

	t.Run("writer status passes through", func(t *testing.T) {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, newDeliveryRequest(
			mintDelivery(t, minter, "svc-pepper", "wh-writer-1", body), body, "svc-pepper"))
		assert.Equal(t, http.StatusForbidden, rec.Code,
			"the writer handler's own status must pass through with no double-200")
		assert.Equal(t, 1, writerCalls,
			"the writer handler must run exactly once for a matching claim")
	})

	t.Run("service-claim mismatch answers 401 before the writer", func(t *testing.T) {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, newDeliveryRequest(
			mintDelivery(t, minter, "svc-other", "wh-writer-2", body), body, "svc-pepper"))
		assert.Equal(t, http.StatusUnauthorized, rec.Code,
			"the service-claim check must win before the writer branch")
		assert.Equal(t, 1, writerCalls,
			"a mismatched claim must never reach the writer handler")
	})
}

// TestHandleDeliveryUnregisteredListingNotFound covers assertion 2: a
// delivery naming a listing with no registered handler is answered 404 and
// never dispatched.
//
// Parameters: t is the test harness.
// Returns: nothing; fails on any deviation.
func TestHandleDeliveryUnregisteredListingNotFound(t *testing.T) {
	minter, mw := newDeliveryTestEnv(t)
	ctrl := NewFederationController(zap.NewNop())

	var dispatched int
	ctrl.RegisterDeliveryHandler("svc-known", func(_ *http.Request) error {
		dispatched++
		return nil
	})

	body := []byte(`{"event":"delivery.ping"}`)
	h := newDeliveryRoute(mw, ctrl)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, newDeliveryRequest(
		mintDelivery(t, minter, "svc-ghost", "wh-delivery-2", body), body, "svc-ghost"))

	assert.Equal(t, http.StatusNotFound, rec.Code,
		"an unregistered listing must be answered 404")
	assert.Zero(t, dispatched,
		"an unregistered listing must not dispatch to any handler")
}
