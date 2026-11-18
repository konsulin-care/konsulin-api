package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/konsulin-care/konsulin-api/internal/app/services/core/referrals"
	"github.com/konsulin-care/konsulin-api/internal/pkg/constvars"
	"github.com/konsulin-care/konsulin-api/internal/pkg/exceptions"
	"github.com/konsulin-care/konsulin-api/internal/pkg/fhir_http_client"
	"github.com/konsulin-care/konsulin-api/internal/pkg/utils"

	"github.com/casbin/casbin/v2"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

// referralBodyMaxBytes bounds the client body read of one referral request.
const referralBodyMaxBytes = 1 << 20

// Referral session paths: the exact Casbin enforce paths the session
// routes authorize against (the same full-path style the services router
// enforces on; the confirm/reject paths carry the live Task id).
const (
	referralSendPath       = "/api/v1/referrals"
	referralIncomingPath   = "/api/v1/referrals/incoming"
	referralConfirmSuffix  = "/confirm"
	referralRejectSuffix   = "/reject"
	referralFHIRResource   = "Task"
	referralRequestedState = "requested"
)

// referralTaskIDKey is the JSON field name every referral response uses
// for the Task id: one literal shared by the three responses (goconst).
const referralTaskIDKey = "taskId"

// IncomingReferral is one inbox row served by GET /api/v1/referrals/incoming:
// the incoming Task waiting for human confirmation plus its soft-match
// candidates. Candidates are references only — never a link.
type IncomingReferral struct {
	TaskID      string   `json:"taskId"`
	Status      string   `json:"status"`
	Description string   `json:"description,omitempty"`
	Candidates  []string `json:"candidates,omitempty"`
}

// ReferralUsecase is the referral workflow seam ReferralController
// dispatches to. registerFederation constructs the implementation over the
// referrals Sender, Receiver and Confirmer (local FHIR resource client,
// partner directory, gateway approval and identity clients). Every method
// returns the operation's result or an error to render through
// utils.BuildErrorResponse; never panics.
type ReferralUsecase interface {
	// SendReferral delivers one outgoing referral and returns the recorded
	// outgoing Task id. caller is the requester's reference, patientID the
	// local Patient, recipientParty the recipient party identifier,
	// serviceRequest the FHIR ServiceRequest JSON and purpose the literal
	// handoff reason.
	SendReferral(ctx context.Context, caller, patientID, recipientParty string,
		serviceRequest json.RawMessage, purpose string) (string, error)
	// ReceiveReferral accepts one inbound referral: it verifies the
	// presented ticket for the referral scope, opens the bound envelope and
	// stores the incoming Task. Returns the stored Task id.
	ReceiveReferral(ctx context.Context, ticket, sealed string) (string, error)
	// Confirm applies the human confirmation for one incoming referral and
	// returns the created local ServiceRequest id.
	Confirm(ctx context.Context, taskID, patientID string) (string, error)
	// Reject marks one incoming referral Task rejected without linking.
	Reject(ctx context.Context, taskID string) error
	// ListIncoming returns the Tasks awaiting confirmation with their
	// soft-match candidates.
	ListIncoming(ctx context.Context) ([]IncomingReferral, error)
}

// referralUsecase adapts the referrals Sender, Receiver and Confirmer plus
// the incoming inbox read to the ReferralUsecase seam.
type referralUsecase struct {
	sender    *referrals.Sender
	receiver  *referrals.Receiver
	confirmer *referrals.Confirmer
	fhir      *fhir_http_client.FHIRHTTPClient
	fhirBase  string
	log       *zap.Logger
}

// NewReferralUsecase wraps the referrals workflow collaborators in the
// ReferralUsecase seam the controller consumes. sender, receiver and
// confirmer may be nil (the corresponding operation then fails closed
// inside the referrals package); fhir and fhirBase back the inbox read.
// Returns the usecase ready to mount. Never panics.
func NewReferralUsecase(sender *referrals.Sender, receiver *referrals.Receiver,
	confirmer *referrals.Confirmer, fhir *fhir_http_client.FHIRHTTPClient,
	fhirBase string, logger *zap.Logger,
) ReferralUsecase {
	return &referralUsecase{
		sender:    sender,
		receiver:  receiver,
		confirmer: confirmer,
		fhir:      fhir,
		fhirBase:  strings.TrimRight(fhirBase, "/"),
		log:       logger,
	}
}

// SendReferral delegates to the referrals Sender. Never panics.
func (u *referralUsecase) SendReferral(ctx context.Context, caller, patientID, recipientParty string,
	serviceRequest json.RawMessage, purpose string,
) (string, error) {
	return u.sender.SendReferral(ctx, caller, patientID, recipientParty, serviceRequest, purpose)
}

// ReceiveReferral delegates to the referrals Receiver. Never panics.
func (u *referralUsecase) ReceiveReferral(ctx context.Context, ticket, sealed string) (string, error) {
	return u.receiver.ReceiveReferral(ctx, ticket, sealed)
}

// Confirm delegates to the referrals Confirmer. Never panics.
func (u *referralUsecase) Confirm(ctx context.Context, taskID, patientID string) (string, error) {
	return u.confirmer.Confirm(ctx, taskID, patientID)
}

// Reject delegates to the referrals Confirmer. Never panics.
func (u *referralUsecase) Reject(ctx context.Context, taskID string) error {
	return u.confirmer.Reject(ctx, taskID)
}

// inboxTask is the incoming FHIR Task subset the inbox read decodes.
type inboxTask struct {
	ID          string `json:"id,omitempty"`
	Status      string `json:"status"`
	Description string `json:"description,omitempty"`
	Input       []struct {
		ValueReference struct {
			Reference string `json:"reference"`
		} `json:"valueReference"`
	} `json:"input,omitempty"`
}

// ListIncoming reads the local FHIR Tasks in status "requested" and maps
// them to inbox rows with their soft-match candidate references.
// Returns the rows and nil, or the error — an unconfigured usecase or a
// failed FHIR search. Never panics.
func (u *referralUsecase) ListIncoming(ctx context.Context) ([]IncomingReferral, error) {
	if u == nil || u.fhir == nil || u.fhirBase == "" {
		return nil, errors.New("referrals: inbox not configured")
	}
	log := u.log
	if log == nil {
		log = zap.NewNop()
	}
	searchURL := u.fhirBase + "/" + referralFHIRResource +
		"?status=" + referralRequestedState
	tasks, err := fhir_http_client.SearchResources[inboxTask](ctx, log, u.fhir,
		searchURL, referralFHIRResource)
	if err != nil {
		return nil, err
	}
	out := make([]IncomingReferral, 0, len(tasks))
	for _, task := range tasks {
		row := IncomingReferral{
			TaskID:      task.ID,
			Status:      task.Status,
			Description: task.Description,
		}
		for _, entry := range task.Input {
			if ref := strings.TrimSpace(entry.ValueReference.Reference); ref != "" {
				row.Candidates = append(row.Candidates, ref)
			}
		}
		out = append(out, row)
	}
	return out, nil
}

// ReferralController serves the referral workflow: the public
// ticket-verified inbound receive (POST /api/v1/federation/referrals) and
// the session + Casbin outgoing send, inbox list, confirm and reject
// routes. Every route sits behind RequireGateway; a missing usecase fails
// closed with the 503 gateway envelope instead of panicking.
type ReferralController struct {
	Log      *zap.Logger
	Usecase  ReferralUsecase
	Enforcer *casbin.Enforcer
}

// NewReferralController builds the referral controller over the usecase
// seam. Logger and usecase may be nil (the handlers then fail closed with
// the 503 gateway envelope). Returns a controller ready to mount.
// Never panics.
func NewReferralController(logger *zap.Logger, usecase ReferralUsecase) *ReferralController {
	return &ReferralController{Log: logger, Usecase: usecase}
}

// Receive serves POST /api/v1/federation/referrals (public,
// ticket-verified): the bearer ticket off the Authorization header and the
// sealed envelope off the {"payload"} body are handed to ReceiveReferral,
// which verifies the ticket and stores the incoming Task; a stored Task
// answers 202 with its id. A rejected referral answers 400, a missing
// ticket or envelope 400, and a missing usecase 503.
// Parameters: w receives the envelope; r carries the header and body.
// Never panics.
func (ctrl *ReferralController) Receive(w http.ResponseWriter, r *http.Request) {
	log := ctrl.logger()
	if ctrl == nil || ctrl.Usecase == nil {
		utils.BuildErrorResponse(log, w, gatewayUnavailable())
		return
	}
	ticket := bearerToken(r.Header.Get(constvars.HeaderAuthorization))
	if ticket == "" {
		utils.BuildErrorResponse(log, w,
			referralBadRequest("Missing referral ticket", "REFERRAL_TICKET_MISSING"))
		return
	}
	sealed, err := readReferralEnvelope(r)
	if err != nil {
		utils.BuildErrorResponse(log, w, err)
		return
	}
	taskID, err := ctrl.Usecase.ReceiveReferral(r.Context(), ticket, sealed)
	if err != nil {
		if errors.Is(err, referrals.ErrReferralRejected) {
			utils.BuildErrorResponse(log, w,
				referralBadRequest("Referral rejected", "REFERRAL_REJECTED"))
			return
		}
		utils.BuildErrorResponse(log, w, err)
		return
	}
	utils.BuildSuccessResponse(w, constvars.StatusAccepted, constvars.ResponseSuccess,
		map[string]string{referralTaskIDKey: taskID})
}

// Send serves POST /api/v1/referrals (session + Casbin): after the
// session/authorization guard it hands the caller, patient, recipient,
// purpose and ServiceRequest body to SendReferral and answers 202 with the
// recorded outgoing Task id. Parameters: w receives the envelope; r
// carries the caller uid, roles and body. Never panics.
func (ctrl *ReferralController) Send(w http.ResponseWriter, r *http.Request) {
	log := ctrl.logger()
	if ctrl == nil || ctrl.Usecase == nil {
		utils.BuildErrorResponse(log, w, gatewayUnavailable())
		return
	}
	if !ctrl.authorizeSession(w, r, http.MethodPost, referralSendPath) {
		return
	}
	var body struct {
		PatientID      string          `json:"patientId"`
		Recipient      string          `json:"recipient"`
		Purpose        string          `json:"purpose"`
		ServiceRequest json.RawMessage `json:"serviceRequest"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, referralBodyMaxBytes)).Decode(&body); err != nil {
		utils.BuildErrorResponse(log, w, exceptions.ErrReadBody(err))
		return
	}
	taskID, err := ctrl.Usecase.SendReferral(r.Context(), serviceCallerRef(r.Context()),
		body.PatientID, body.Recipient, body.ServiceRequest, body.Purpose)
	if err != nil {
		utils.BuildErrorResponse(log, w, err)
		return
	}
	utils.BuildSuccessResponse(w, constvars.StatusAccepted, constvars.ResponseSuccess,
		map[string]string{referralTaskIDKey: taskID})
}

// ListIncoming serves GET /api/v1/referrals/incoming (session + Casbin):
// the guarded inbox of Tasks awaiting confirmation with their soft-match
// candidates. Parameters: w receives the envelope; r carries the caller
// uid and roles. Never panics.
func (ctrl *ReferralController) ListIncoming(w http.ResponseWriter, r *http.Request) {
	log := ctrl.logger()
	if ctrl == nil || ctrl.Usecase == nil {
		utils.BuildErrorResponse(log, w, gatewayUnavailable())
		return
	}
	if !ctrl.authorizeSession(w, r, http.MethodGet, referralIncomingPath) {
		return
	}
	inbox, err := ctrl.Usecase.ListIncoming(r.Context())
	if err != nil {
		utils.BuildErrorResponse(log, w, err)
		return
	}
	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.ResponseSuccess, inbox)
}

// Confirm serves POST /api/v1/referrals/incoming/{id}/confirm (session +
// Casbin): the guarded human confirmation of the {id} Task for the patient
// named in the {patientId} body, answering 200 with the created local
// ServiceRequest id. Parameters: w receives the envelope; r carries the
// route param, caller uid, roles and body. Never panics.
func (ctrl *ReferralController) Confirm(w http.ResponseWriter, r *http.Request) {
	log := ctrl.logger()
	if ctrl == nil || ctrl.Usecase == nil {
		utils.BuildErrorResponse(log, w, gatewayUnavailable())
		return
	}
	taskID := strings.TrimSpace(chi.URLParam(r, "id"))
	if !ctrl.authorizeSession(w, r, http.MethodPost,
		referralIncomingPath+"/"+taskID+referralConfirmSuffix) {
		return
	}
	var body struct {
		PatientID string `json:"patientId"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, referralBodyMaxBytes)).Decode(&body); err != nil {
		utils.BuildErrorResponse(log, w, exceptions.ErrReadBody(err))
		return
	}
	serviceRequestID, err := ctrl.Usecase.Confirm(r.Context(), taskID, body.PatientID)
	if err != nil {
		utils.BuildErrorResponse(log, w, err)
		return
	}
	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.ResponseSuccess,
		map[string]string{"serviceRequestId": serviceRequestID})
}

// Reject serves POST /api/v1/referrals/incoming/{id}/reject (session +
// Casbin): the guarded rejection of the {id} Task without any identity
// link, answering 200 with the rejected Task id. Parameters: w receives
// the envelope; r carries the route param, caller uid and roles.
// Never panics.
func (ctrl *ReferralController) Reject(w http.ResponseWriter, r *http.Request) {
	log := ctrl.logger()
	if ctrl == nil || ctrl.Usecase == nil {
		utils.BuildErrorResponse(log, w, gatewayUnavailable())
		return
	}
	taskID := strings.TrimSpace(chi.URLParam(r, "id"))
	if !ctrl.authorizeSession(w, r, http.MethodPost,
		referralIncomingPath+"/"+taskID+referralRejectSuffix) {
		return
	}
	if err := ctrl.Usecase.Reject(r.Context(), taskID); err != nil {
		utils.BuildErrorResponse(log, w, err)
		return
	}
	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.ResponseSuccess,
		map[string]string{referralTaskIDKey: taskID})
}

// authorizeSession guards the session + Casbin referral routes: a caller
// without a session uid is rejected 401 and a caller whose roles Casbin
// does not grant on the exact method/path are rejected 403 (fail closed
// when the enforcer or the roles are missing, mirroring the services
// router). The refusal is written to w. Returns whether the request may
// proceed. Never panics.
func (ctrl *ReferralController) authorizeSession(w http.ResponseWriter, r *http.Request,
	method, path string,
) bool {
	if serviceCallerRef(r.Context()) == "" {
		utils.BuildErrorResponse(ctrl.logger(), w,
			exceptions.BuildNewCustomError(nil, constvars.StatusUnauthorized,
				"Authentication required", "UNAUTHENTICATED"))
		return false
	}
	roles := serviceRoles(r.Context())
	if ctrl.Enforcer == nil {
		utils.BuildErrorResponse(ctrl.logger(), w, referralForbidden())
		return false
	}
	for _, role := range roles {
		normalized := strings.TrimSpace(role)
		if normalized == "" {
			continue
		}
		ok, err := ctrl.Enforcer.Enforce(normalized, method, path)
		if err == nil && ok {
			return true
		}
	}
	utils.BuildErrorResponse(ctrl.logger(), w, referralForbidden())
	return false
}

// readReferralEnvelope decodes the {"payload": sealed} body the referrals
// Sender posts and trims it. Returns the compact JWE and nil, or a 400
// renderable error when the body is unreadable or carries no envelope.
// Never panics.
func readReferralEnvelope(r *http.Request) (string, error) {
	var body struct {
		Payload string `json:"payload"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, referralBodyMaxBytes)).Decode(&body); err != nil {
		return "", exceptions.ErrReadBody(err)
	}
	sealed := strings.TrimSpace(body.Payload)
	if sealed == "" {
		return "", referralBadRequest("Missing referral envelope", "REFERRAL_ENVELOPE_MISSING")
	}
	return sealed, nil
}

// bearerToken extracts the token off an "Authorization: Bearer <token>"
// header. Returns "" for a missing or differently shaped header.
// Never panics.
func bearerToken(header string) string {
	scheme, token, found := strings.Cut(strings.TrimSpace(header), " ")
	if !found || !strings.EqualFold(scheme, "bearer") {
		return ""
	}
	return strings.TrimSpace(token)
}

// referralBadRequest renders one 400 for a malformed referral request.
func referralBadRequest(message, dev string) error {
	return exceptions.BuildNewCustomError(nil, constvars.StatusBadRequest, message, dev)
}

// referralForbidden renders the fail-closed 403 for a caller Casbin does
// not grant the referral path.
func referralForbidden() error {
	return exceptions.BuildNewCustomError(nil, constvars.StatusForbidden,
		"Referral not permitted", "REFERRAL_FORBIDDEN")
}

// logger returns the controller's logger, falling back to a no-op logger
// for a nil controller or logger. Never panics.
func (ctrl *ReferralController) logger() *zap.Logger {
	if ctrl != nil && ctrl.Log != nil {
		return ctrl.Log
	}
	return zap.NewNop()
}
