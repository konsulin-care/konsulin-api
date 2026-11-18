package controllers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/konsulin-care/konsulin-api/internal/app/delivery/http/middlewares"
	"github.com/konsulin-care/konsulin-api/internal/pkg/constvars"
	"github.com/konsulin-care/konsulin-api/internal/pkg/exceptions"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// fakeStudyWithdrawer records Withdraw calls.
type fakeStudyWithdrawer struct {
	patients []string
	studies  []string
	err      error
}

// Withdraw records the patient and study and returns the configured error.
func (f *fakeStudyWithdrawer) Withdraw(_ context.Context, patient, study string) error {
	f.patients = append(f.patients, patient)
	f.studies = append(f.studies, study)
	return f.err
}

// researchDisabledGatewayStatus reports a disabled gateway (standalone
// mode) so RequireGateway answers 503 before any handler runs.
type researchDisabledGatewayStatus struct{}

// Enabled reports the gateway as not configured.
func (researchDisabledGatewayStatus) Enabled() bool { return false }

// Enrolled reports the party as not enrolled.
func (researchDisabledGatewayStatus) Enrolled() bool { return false }

// researchRequest builds a request carrying the caller uid and roles the
// handlers read from context.
func researchRequest(uid, method, target, body string, roles ...string) *http.Request {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	ctx := context.WithValue(req.Context(), constvars.CONTEXT_FHIR_ROLE, roles)
	ctx = context.WithValue(ctx, constvars.CONTEXT_UID, uid)
	return req.WithContext(ctx)
}

// TestWithdrawStudyInvokesWithdrawalForCallersPatient covers the first T76
// assertion: POST /api/v1/research/studies/{id}/withdraw runs the
// withdrawal flow with the caller's session uid as the patient and the
// path id as the study — never a client-supplied patient.
func TestWithdrawStudyInvokesWithdrawalForCallersPatient(t *testing.T) {
	fake := &fakeStudyWithdrawer{}
	ctrl := NewResearchController(zap.NewNop(), fake, nil, nil)
	mux := chi.NewRouter()
	mux.Post("/research/studies/{id}/withdraw", ctrl.WithdrawStudy)

	req := researchRequest("pat-9", http.MethodPost,
		"/research/studies/st-1/withdraw", "", constvars.KonsulinRolePatient)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, []string{"pat-9"}, fake.patients,
		"withdrawal must run for the caller's patient")
	require.Equal(t, []string{"st-1"}, fake.studies,
		"withdrawal must run for the study in the path")
}

// TestResearchExportRoutesStandaloneAnswer503NotConfigured covers the
// second T76 assertion: with the gateway disabled every export/request
// route answers 503 with dev_message GATEWAY_NOT_CONFIGURED because
// RequireGateway runs on the mount before any handler.
func TestResearchExportRoutesStandaloneAnswer503NotConfigured(t *testing.T) {
	// BuildErrorResponse only exposes dev_message outside production.
	t.Setenv("APP_ENV", "development")
	ctrl := NewResearchController(zap.NewNop(), &fakeStudyWithdrawer{}, nil, nil)
	mux := chi.NewRouter()
	r := mux.With(middlewares.RequireGateway(researchDisabledGatewayStatus{}))
	r.Post("/research/requests", ctrl.RequestExport)
	r.Get("/research/requests", ctrl.ListRequests)
	r.Post("/federation/research/exports", ctrl.ReceiveExport)
	r.Get("/federation/research/exports/downloads/{token}", ctrl.DownloadExport)

	for _, tc := range []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/research/requests"},
		{http.MethodGet, "/research/requests"},
		{http.MethodPost, "/federation/research/exports"},
		{http.MethodGet, "/federation/research/exports/downloads/tok-1"},
	} {
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(""))
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
}
