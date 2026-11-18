package controllers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/konsulin-care/konsulin-api/internal/app/contracts"
	"github.com/konsulin-care/konsulin-api/internal/app/delivery/http/middlewares"
	"github.com/konsulin-care/konsulin-api/internal/pkg/exceptions"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// referralFakeUsecase records what the controller presents so the tests
// can prove which seams ran. The unused operations answer zero values.
type referralFakeUsecase struct {
	receiveCalls   int
	receivedTicket string
	receivedSealed string
}

// SendReferral is never exercised by these tests.
func (f *referralFakeUsecase) SendReferral(context.Context, string, string, string, json.RawMessage, string) (string, error) {
	return "", nil
}

// ReceiveReferral records the ticket and sealed envelope it was handed.
func (f *referralFakeUsecase) ReceiveReferral(_ context.Context, ticket, sealed string) (string, error) {
	f.receiveCalls++
	f.receivedTicket = ticket
	f.receivedSealed = sealed
	return "task-42", nil
}

// Confirm is never exercised by these tests.
func (f *referralFakeUsecase) Confirm(context.Context, string, string) (string, error) {
	return "", nil
}

// Reject is never exercised by these tests.
func (f *referralFakeUsecase) Reject(context.Context, string) error { return nil }

// ListIncoming is never exercised by these tests.
func (f *referralFakeUsecase) ListIncoming(context.Context) ([]IncomingReferral, error) {
	return nil, nil
}

// newReferralTestMux mirrors routers.attachReferralRouter: every referral
// route behind RequireGateway over an arbitrary gateway status, so an
// enabled status reaches the handlers and a disabled status (standalone
// mode) answers 503 before any handler runs.
func newReferralTestMux(ctrl *ReferralController, status contracts.GatewayStatus) *chi.Mux {
	mux := chi.NewRouter()
	r := mux.With(middlewares.RequireGateway(status))
	r.Post("/federation/referrals", ctrl.Receive)
	r.Post("/referrals", ctrlsSend(ctrl))
	r.Get("/referrals/incoming", ctrl.ListIncoming)
	r.Post("/referrals/incoming/{id}/confirm", ctrl.Confirm)
	r.Post("/referrals/incoming/{id}/reject", ctrl.Reject)
	return mux
}

// ctrlsSend keeps the mux construction readable; it is the same handler
// attachReferralRouter binds for POST /referrals.
func ctrlsSend(ctrl *ReferralController) http.HandlerFunc {
	return ctrl.Send
}

// TestReceiveReferralRouteHandsTicketToUseCaseAndAnswers202 covers the
// first T66 assertion: a ticket-bearing POST /api/v1/federation/referrals
// hands the bearer ticket and the sealed payload to ReceiveReferral and
// answers 202 with the stored Task id.
func TestReceiveReferralRouteHandsTicketToUseCaseAndAnswers202(t *testing.T) {
	// BuildErrorResponse only exposes dev_message outside production.
	t.Setenv("APP_ENV", "development")
	usecase := &referralFakeUsecase{}
	mux := newReferralTestMux(NewReferralController(zap.NewNop(), usecase),
		servicesFakeGatewayStatus{})

	req := httptest.NewRequest(http.MethodPost, "/federation/referrals",
		strings.NewReader(`{"payload":"sealed.referral.jwe"}`))
	req.Header.Set("Authorization", "Bearer ks-approval-ticket")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusAccepted, rec.Code, rec.Body.String())
	require.Equal(t, 1, usecase.receiveCalls, "exactly one ReceiveReferral call")
	assert.Equal(t, "ks-approval-ticket", usecase.receivedTicket,
		"the bearer ticket must reach ReceiveReferral")
	assert.Equal(t, "sealed.referral.jwe", usecase.receivedSealed,
		"the sealed payload must reach ReceiveReferral")
	var payload struct {
		Success bool `json:"success"`
		Data    struct {
			TaskID string `json:"taskId"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	require.True(t, payload.Success)
	assert.Equal(t, "task-42", payload.Data.TaskID,
		"the stored Task id must be echoed in the 202 body")
}

// TestReferralRoutesStandaloneAnswer503NotConfigured covers the second
// T66 assertion: with the gateway disabled every referral route answers
// 503 with dev_message GATEWAY_NOT_CONFIGURED because RequireGateway runs
// on the mount before any handler.
func TestReferralRoutesStandaloneAnswer503NotConfigured(t *testing.T) {
	// BuildErrorResponse only exposes dev_message outside production.
	t.Setenv("APP_ENV", "development")
	usecase := &referralFakeUsecase{}
	mux := newReferralTestMux(NewReferralController(zap.NewNop(), usecase),
		servicesDisabledGatewayStatus{})

	for _, tc := range []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/federation/referrals"},
		{http.MethodPost, "/referrals"},
		{http.MethodGet, "/referrals/incoming"},
		{http.MethodPost, "/referrals/incoming/task-1/confirm"},
		{http.MethodPost, "/referrals/incoming/task-1/reject"},
	} {
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader("{}"))
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		require.Equal(t, http.StatusServiceUnavailable, w.Code,
			tc.path+": standalone mode must answer 503")
		var envelope exceptions.CustomError
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &envelope),
			tc.path+": the error response must be a CustomError JSON envelope")
		assert.Equal(t, "GATEWAY_NOT_CONFIGURED", envelope.DevMessage,
			tc.path+": dev_message must be the GATEWAY_NOT_CONFIGURED code")
	}
	assert.Equal(t, 0, usecase.receiveCalls,
		"no handler may run in standalone mode")
}
