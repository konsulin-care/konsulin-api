package controllers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/utils"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

// StudyWithdrawer withdraws one patient from one study (BK-DATABANK-003);
// *research.Withdrawer satisfies it in production wiring.
type StudyWithdrawer interface {
	// Withdraw marks the patient's ResearchSubjects withdrawn, disables
	// their research consents and deletes their derived research rows for
	// the given study.
	Withdraw(ctx context.Context, patient, study string) error
}

// ExportRequester runs one researcher's cross-party export request
// (BK-DATABANK-004); *research.Requester satisfies it in production
// wiring.
type ExportRequester interface {
	// RequestExport returns the partner-issued export id.
	RequestExport(ctx context.Context, researcher, partnerParty, study string) (string, error)
}

// ResearchExporter serves the inbound export protocol (BK-DATABANK-004);
// *research.Exporter satisfies it in production wiring.
type ResearchExporter interface {
	// StartExport verifies the presented ticket, opens the sealed request
	// and returns the single-use download token for the async job.
	StartExport(ctx context.Context, ticket, sealed string) (string, error)
	// Download consumes one download token and returns the sealed archive.
	Download(ctx context.Context, token string) (string, error)
}

// ResearchController serves the research databank HTTP surface: patient
// withdrawal, researcher export requests and the ticket-verified inbound
// export/download pair. A missing collaborator fails closed with 503
// instead of panicking.
type ResearchController struct {
	Log        *zap.Logger
	Withdrawer StudyWithdrawer
	Requester  ExportRequester
	Exporter   ResearchExporter
}

// NewResearchController constructs the research controller over its
// collaborators; nil collaborators make the matching handlers fail closed.
// Parameters: logger reports handler failures (nil selects a no-op
// logger), withdrawer runs patient withdrawal, requester runs researcher
// export requests, exporter serves the inbound export protocol.
// Returns the controller to mount. Never panics.
func NewResearchController(logger *zap.Logger, withdrawer StudyWithdrawer, requester ExportRequester, exporter ResearchExporter) *ResearchController {
	return &ResearchController{
		Log:        logger,
		Withdrawer: withdrawer,
		Requester:  requester,
		Exporter:   exporter,
	}
}

// logger returns the controller's logger, falling back to a no-op logger.
// Never panics.
func (ctrl *ResearchController) logger() *zap.Logger {
	if ctrl != nil && ctrl.Log != nil {
		return ctrl.Log
	}
	return zap.NewNop()
}

// researchUnavailable answers 503 when a required research collaborator
// is missing from the deployment wiring (databank disabled or gateway
// seams absent).
func researchUnavailable() error {
	return exceptions.BuildNewCustomError(nil, constvars.StatusServiceUnavailable,
		"Research databank not configured", "RESEARCH_NOT_CONFIGURED")
}

// researchForbidden answers 403 when the caller's session roles do not
// include the route's required role.
func researchForbidden() error {
	return exceptions.BuildNewCustomError(nil, http.StatusForbidden,
		"Forbidden", "RESEARCH_FORBIDDEN")
}

// researchHasRole reports whether the request's CONTEXT_FHIR_ROLE list
// contains role (case-insensitive); a missing context value fails closed.
// Never panics.
func researchHasRole(r *http.Request, role string) bool {
	roles, _ := r.Context().Value(constvars.CONTEXT_FHIR_ROLE).([]string)
	for _, candidate := range roles {
		if strings.EqualFold(strings.TrimSpace(candidate), role) {
			return true
		}
	}
	return false
}

// researchTicket extracts the bearer ticket from the Authorization header,
// tolerating any casing of the "Bearer " prefix. Returns "" when absent.
// Never panics.
func researchTicket(r *http.Request) string {
	header := strings.TrimSpace(r.Header.Get("Authorization"))
	const prefix = "bearer "
	if len(header) < len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
		return ""
	}
	return strings.TrimSpace(header[len(prefix):])
}

// WithdrawStudy serves POST /api/v1/research/studies/{id}/withdraw
// (Patient): the caller's patient (session uid) withdraws from the study
// in the path, invoking the withdrawal flow with that uid — never a
// client-supplied patient. A missing session, a non-Patient role or a
// disabled databank fails closed before the withdrawal runs.
// Parameters: w receives the envelope; r carries the id route param, the
// caller uid and roles. Never panics.
func (ctrl *ResearchController) WithdrawStudy(w http.ResponseWriter, r *http.Request) {
	log := ctrl.logger()
	uid, _ := r.Context().Value(constvars.CONTEXT_UID).(string)
	if uid == "" || strings.EqualFold(uid, "anonymous") {
		utils.BuildErrorResponse(log, w, exceptions.ErrSupertokensSessionMissing(nil))
		return
	}
	if !researchHasRole(r, constvars.KonsulinRolePatient) {
		utils.BuildErrorResponse(log, w, researchForbidden())
		return
	}
	if ctrl == nil || ctrl.Withdrawer == nil {
		utils.BuildErrorResponse(log, w, researchUnavailable())
		return
	}
	study := strings.TrimSpace(chi.URLParam(r, "id"))
	if err := ctrl.Withdrawer.Withdraw(r.Context(), uid, study); err != nil {
		utils.BuildErrorResponse(log, w, err)
		return
	}
	utils.BuildSuccessResponse(w, http.StatusOK, "withdrawn", nil)
}

// exportRequestBody is the body of POST /api/v1/research/requests.
type exportRequestBody struct {
	Researcher string `json:"researcher"`
	Partner    string `json:"partner"`
	Study      string `json:"study"`
}

// RequestExport serves POST /api/v1/research/requests (Researcher): it
// runs the cross-party export request with the caller's researcher
// identity (session uid when the body omits "researcher"), the partner
// party and the study, answering 202 with the partner-issued export id.
// A non-Researcher caller or a missing requester fails closed.
// Parameters: w receives the envelope; r carries the JSON body and the
// caller roles. Never panics.
func (ctrl *ResearchController) RequestExport(w http.ResponseWriter, r *http.Request) {
	log := ctrl.logger()
	if !researchHasRole(r, constvars.KonsulinRoleResearcher) {
		utils.BuildErrorResponse(log, w, researchForbidden())
		return
	}
	if ctrl == nil || ctrl.Requester == nil {
		utils.BuildErrorResponse(log, w, researchUnavailable())
		return
	}
	var body exportRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		utils.BuildErrorResponse(log, w, exceptions.ErrCannotParseJSON(err))
		return
	}
	researcher := strings.TrimSpace(body.Researcher)
	if researcher == "" {
		researcher, _ = r.Context().Value(constvars.CONTEXT_UID).(string)
	}
	id, err := ctrl.Requester.RequestExport(r.Context(), researcher,
		strings.TrimSpace(body.Partner), strings.TrimSpace(body.Study))
	if err != nil {
		utils.BuildErrorResponse(log, w, err)
		return
	}
	utils.BuildSuccessResponse(w, http.StatusAccepted, "requested", id)
}

// ListRequests serves GET /api/v1/research/requests (Researcher). No
// request-listing store exists yet (the archive store keeps no index), so
// the handler fails closed with 503 instead of inventing a listing; a
// later task adds the lister seam here.
// Parameters: w receives the envelope; r carries the caller roles.
// Never panics.
func (ctrl *ResearchController) ListRequests(w http.ResponseWriter, r *http.Request) {
	log := ctrl.logger()
	if !researchHasRole(r, constvars.KonsulinRoleResearcher) {
		utils.BuildErrorResponse(log, w, researchForbidden())
		return
	}
	utils.BuildErrorResponse(log, w, researchUnavailable())
}

// exportEnvelope is the inbound export POST body: the requester's "payload"
// field carries the compact bound JWE holding {study}.
type exportEnvelope struct {
	Payload string `json:"payload"`
}

// ReceiveExport serves POST /api/v1/federation/research/exports
// (ticket-verified): the caller presents the approval ticket as a bearer
// credential, the Exporter verifies it, opens the sealed {study} request
// and starts the async job. The answer is raw JSON {"token": ...} with
// 202 — the requesting party's requester decodes it directly — carrying
// the single-use 24h download token.
// Parameters: w receives the raw answer; r carries the Authorization
// header and the JSON body. Never panics.
func (ctrl *ResearchController) ReceiveExport(w http.ResponseWriter, r *http.Request) {
	log := ctrl.logger()
	if ctrl == nil || ctrl.Exporter == nil {
		utils.BuildErrorResponse(log, w, researchUnavailable())
		return
	}
	var body exportEnvelope
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		utils.BuildErrorResponse(log, w, exceptions.ErrCannotParseJSON(err))
		return
	}
	ticket := researchTicket(r)
	if ticket == "" {
		utils.BuildErrorResponse(log, w, exceptions.BuildNewCustomError(nil,
			http.StatusUnauthorized, "Missing export ticket", "RESEARCH_TICKET_MISSING"))
		return
	}
	token, err := ctrl.Exporter.StartExport(r.Context(), ticket, strings.TrimSpace(body.Payload))
	if err != nil {
		utils.BuildErrorResponse(log, w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]string{"token": token})
}

// DownloadExport serves GET /api/v1/federation/research/exports/
// downloads/{token}: the single-use 24h token consumes the sealed archive,
// written as the raw compact JWE (application/jose) the requesting party
// opens itself; an unknown, consumed or expired token fails closed.
// Parameters: w receives the sealed body; r carries the token route param.
// Never panics.
func (ctrl *ResearchController) DownloadExport(w http.ResponseWriter, r *http.Request) {
	log := ctrl.logger()
	if ctrl == nil || ctrl.Exporter == nil {
		utils.BuildErrorResponse(log, w, researchUnavailable())
		return
	}
	token := strings.TrimSpace(chi.URLParam(r, "token"))
	sealed, err := ctrl.Exporter.Download(r.Context(), token)
	if err != nil {
		utils.BuildErrorResponse(log, w, err)
		return
	}
	w.Header().Set("Content-Type", "application/jose")
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, sealed)
}
