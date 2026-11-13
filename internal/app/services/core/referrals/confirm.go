package referrals

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/app/services/core/federation"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/fhir_dto"
	"konsulin-service/internal/pkg/fhir_http_client"

	"github.com/konsulin-care/gateway-sdk-go/identity"
	"go.uber.org/zap"
)

// Confirmation status values: an incoming referral Task starts
// "requested" (receive.go) and ends "accepted" or "rejected" here.
const (
	taskStatusAccepted = "accepted"
	taskStatusRejected = "rejected"
)

// IdentityLinker is the identity-plane seam Confirm needs:
// contracts.IdentityClient's Resolve and Link operations, so the human
// confirmation can join the confirmed patient's identity at the gateway.
// The gateway driver's *gateway.Client satisfies it.
type IdentityLinker interface {
	// Resolve resolves peppered contact hashes to candidate identity rows
	// (the confirmed candidate's global key comes from here).
	Resolve(ctx context.Context, emailHash, phoneHash string) ([]contracts.IdentityCandidate, error)
	// Link declares equivalence between the confirmed global key and the
	// party-local resource reference.
	Link(ctx context.Context, globalKey, localResourceID string) (bool, error)
}

// Confirmer applies the human confirmation step of an incoming referral
// (BK-REFERRAL-002): Confirm joins the confirmed candidate's identity at
// the gateway, materializes the local ServiceRequest and accepts the
// Task; Reject marks the Task rejected without ever calling the identity
// plane. Soft matching only surfaced candidates — this type owns the
// explicit link decision.
type Confirmer struct {
	identities  IdentityLinker
	fhir        *fhir_http_client.FHIRHTTPClient
	fhirBaseURL string
	log         *zap.Logger
}

// ConfirmerDeps bundles the collaborators Confirm and Reject need.
type ConfirmerDeps struct {
	// Identities resolves and links identities at the gateway (required).
	Identities IdentityLinker
	// FHIRClient performs the local Task reads/writes, Patient read and
	// ServiceRequest create (required).
	FHIRClient *fhir_http_client.FHIRHTTPClient
	// FHIRBaseURL is the local FHIR server root (required; trailing
	// slashes are trimmed).
	FHIRBaseURL string
	// Logger records confirmation traffic (nil uses a no-op logger).
	Logger *zap.Logger
}

// NewConfirmer constructs the human-confirmation step over its
// collaborators.
//
// Returns a Confirmer ready for Confirm and Reject. Never panics.
func NewConfirmer(deps ConfirmerDeps) *Confirmer {
	log := deps.Logger
	if log == nil {
		log = zap.NewNop()
	}
	return &Confirmer{
		identities:  deps.Identities,
		fhir:        deps.FHIRClient,
		fhirBaseURL: strings.TrimRight(deps.FHIRBaseURL, "/"),
		log:         log,
	}
}

// confirmTask is the incoming Task subset the confirmation reads: the
// status gate, the candidate list, the purpose and the requester.
type confirmTask struct {
	// ID is the Task id.
	ID string `json:"id,omitempty"`
	// ResourceType is always "Task".
	ResourceType string `json:"resourceType"`
	// Status is "requested" until the human decides.
	Status string `json:"status"`
	// Description carries the referral purpose (becomes the local order note).
	Description string `json:"description,omitempty"`
	// Requester is the caller's FHIR reference, kept on the local order.
	Requester *fhir_dto.Reference `json:"requester,omitempty"`
	// Focus references the referred order (the sender's id on intake).
	Focus *fhir_dto.Reference `json:"focus,omitempty"`
	// Input lists the soft-matched candidate patients.
	Input []incomingTaskInput `json:"input,omitempty"`
}

// confirmedServiceRequest is the local FHIR ServiceRequest created when
// the human confirms the candidate: the referral order materialized
// locally so the Task focus resolves against this party's own server.
type confirmedServiceRequest struct {
	// ID is the created ServiceRequest id echoed by the FHIR server.
	ID string `json:"id,omitempty"`
	// ResourceType is always "ServiceRequest".
	ResourceType string `json:"resourceType"`
	// Status is "active" once the hand-off is confirmed.
	Status string `json:"status"`
	// Intent is always "order".
	Intent string `json:"intent"`
	// Subject is the confirmed patient.
	Subject fhir_dto.Reference `json:"subject"`
	// Requester carries the incoming Task's requester when present.
	Requester *fhir_dto.Reference `json:"requester,omitempty"`
	// Note carries the referral purpose from the incoming Task.
	Note []fhir_dto.Annotation `json:"note,omitempty"`
}

// Confirm applies the human confirmation for one incoming referral
// (BK-REFERRAL-002): it reads the Task, requires status "requested" and
// the confirmed patient among its candidates, resolves the patient's
// peppered contact hashes to the confirmed candidate's global key, links
// that identity to the local patient at the gateway (fail closed before
// any local write), creates the local ServiceRequest for the confirmed
// patient, and accepts the Task — assigning Task.for to the patient and
// repointing Task.focus at the created local ServiceRequest.
//
// Parameters:
//   - ctx bounds every FHIR and gateway call.
//   - taskID is the incoming Task to confirm.
//   - patientID is the confirmed candidate Patient id.
//
// Returns the created local ServiceRequest id and nil, or "" and the
// error — an unconfigured confirmer, a missing/not-yet-requested Task, a
// patient outside the candidate list, a resolution or link failure, or a
// FHIR write error. Never panics.
func (c *Confirmer) Confirm(ctx context.Context, taskID, patientID string) (string, error) {
	if c == nil || c.identities == nil || c.fhir == nil || c.fhirBaseURL == "" {
		return "", errors.New("referrals: confirmer not configured")
	}
	taskID = strings.TrimSpace(taskID)
	patientID = strings.TrimSpace(patientID)
	if taskID == "" || patientID == "" {
		return "", errors.New("referrals: missing task or patient")
	}
	raw, err := c.readTask(ctx, taskID)
	if err != nil {
		return "", err
	}
	var task confirmTask
	if err := json.Unmarshal(raw, &task); err != nil {
		return "", fmt.Errorf("referrals: decode task: %w", err)
	}
	if task.Status != taskStatusRequested {
		return "", errors.New("referrals: task is not awaiting confirmation")
	}
	if !isListedCandidate(task.Input, patientID) {
		return "", errors.New("referrals: patient is not a listed candidate")
	}
	globalKey, err := c.resolveGlobalKey(ctx, patientID)
	if err != nil {
		return "", err
	}
	patientRef := constvars.FHIRRefPrefixPatient + patientID
	linked, err := c.identities.Link(ctx, globalKey, patientRef)
	if err != nil {
		return "", fmt.Errorf("referrals: link identity: %w", err)
	}
	if !linked {
		return "", errors.New("referrals: gateway declined the identity link")
	}
	srID, err := c.createServiceRequest(ctx, &task, patientRef)
	if err != nil {
		return "", err
	}
	err = c.writeTask(ctx, taskID, raw, func(doc map[string]any) {
		doc["status"] = taskStatusAccepted
		doc["for"] = map[string]any{"reference": patientRef}
		doc["focus"] = map[string]any{
			"reference": constvars.ResourceServiceRequest + "/" + srID,
		}
	})
	if err != nil {
		return "", err
	}
	return srID, nil
}

// Reject marks one incoming referral Task as rejected: the Task is read,
// its status flipped to "rejected" and written back — the identity plane
// is never touched, so no join can happen on rejection.
//
// Parameters:
//   - ctx bounds the FHIR read and write.
//   - taskID is the incoming Task to reject.
//
// Returns nil once the Task is rejected, or the error — an unconfigured
// confirmer, a missing task, or a FHIR failure. Never panics.
func (c *Confirmer) Reject(ctx context.Context, taskID string) error {
	if c == nil || c.fhir == nil || c.fhirBaseURL == "" {
		return errors.New("referrals: confirmer not configured")
	}
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return errors.New("referrals: missing task")
	}
	raw, err := c.readTask(ctx, taskID)
	if err != nil {
		return err
	}
	return c.writeTask(ctx, taskID, raw, func(doc map[string]any) {
		doc["status"] = taskStatusRejected
	})
}

// readTask fetches the raw local Task JSON. Returns the bytes and nil, or
// the error naming the read. Never panics.
func (c *Confirmer) readTask(ctx context.Context, taskID string) ([]byte, error) {
	target := c.fhirBaseURL + "/" + resourceTask + "/" + taskID
	raw, err := c.fhir.Do(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, fmt.Errorf("referrals: read task: %w", err)
	}
	return raw, nil
}

// writeTask round-trips the raw Task through a generic document so every
// intake field survives the update, applies mutate and PUTs the result
// back (FHIR PUT replaces the resource).
//
// Parameters:
//   - ctx bounds the write, taskID the Task id, raw the previously read
//     Task JSON, mutate applies the status/identity changes in place.
//
// Returns nil once the FHIR server accepts the write, or the error.
// Never panics.
func (c *Confirmer) writeTask(ctx context.Context, taskID string, raw []byte,
	mutate func(map[string]any),
) error {
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return fmt.Errorf("referrals: decode task for update: %w", err)
	}
	mutate(doc)
	_, err := fhir_http_client.WriteResource(fhir_http_client.WriteResourceInput[map[string]any]{
		Ctx:          ctx,
		Log:          c.log,
		Client:       c.fhir,
		Method:       http.MethodPut,
		BaseUrl:      c.fhirBaseURL + "/" + resourceTask,
		ID:           taskID,
		Resource:     &doc,
		ResourceName: resourceTask,
		IDLogKey:     "task_id",
	})
	if err != nil {
		return fmt.Errorf("referrals: write task: %w", err)
	}
	return nil
}

// isListedCandidate reports whether the confirmed patient appears among
// the Task's soft-match candidate entries. Never panics.
func isListedCandidate(inputs []incomingTaskInput, patientID string) bool {
	want := constvars.FHIRRefPrefixPatient + patientID
	for _, in := range inputs {
		if in.ValueReference.Reference == want {
			return true
		}
	}
	return false
}

// resolveGlobalKey reads the confirmed local Patient, hashes its contact
// values with the federation pepper (the gateway "N/A" marker for an
// absent field) and resolves them to the confirmed candidate's global
// key: a candidate row other than the local patient's own is preferred,
// any candidate carrying a global key is accepted, and no candidate
// fails closed before any link.
//
// Parameters:
//   - ctx bounds the Patient read and the gateway resolve.
//   - patientID is the confirmed candidate Patient id.
//
// Returns the global key and nil, or "" and the error. Never panics.
func (c *Confirmer) resolveGlobalKey(ctx context.Context, patientID string) (string, error) {
	patient, err := fhir_http_client.GetResource[fhir_dto.Patient](ctx, c.log, c.fhir,
		c.fhirBaseURL+"/"+constvars.ResourcePatient, patientID,
		constvars.ResourcePatient, "patient_id")
	if err != nil {
		return "", fmt.Errorf("referrals: read patient: %w", err)
	}
	pepper, err := federation.LoadPepper()
	if err != nil {
		return "", fmt.Errorf("referrals: load pepper: %w", err)
	}
	email, phone := referralContact(patient.Telecom)
	if email == "" && phone == "" {
		return "", errors.New("referrals: confirmed patient carries no contact to resolve")
	}
	emailHash, phoneHash, err := hashContact(email, phone, pepper)
	if err != nil {
		return "", err
	}
	candidates, err := c.identities.Resolve(ctx, emailHash, phoneHash)
	if err != nil {
		return "", fmt.Errorf("referrals: resolve identity: %w", err)
	}
	return pickGlobalKey(candidates, constvars.FHIRRefPrefixPatient+patientID)
}

// hashContact hashes the confirmed patient's contact values with the
// federation pepper, using the gateway "N/A" marker for an absent
// field.
//
// Parameters:
//   - email is the patient's email address, empty when absent.
//   - phone is the patient's phone number, empty when absent.
//   - pepper is the federation pepper to hash with.
//
// Returns the email hash, the phone hash and nil, or "", "" and the
// error. Never panics.
func hashContact(email, phone string, pepper []byte) (emailHash, phoneHash string, err error) {
	emailHash, phoneHash = identity.NAMarker, identity.NAMarker
	if email != "" {
		emailHash, err = identity.HashEmail(email, pepper)
		if err != nil {
			return "", "", fmt.Errorf("referrals: hash email: %w", err)
		}
	}
	if phone != "" {
		phoneHash, err = identity.HashPhone(phone, pepper)
		if err != nil {
			return "", "", fmt.Errorf("referrals: hash phone: %w", err)
		}
	}
	return emailHash, phoneHash, nil
}

// pickGlobalKey chooses the confirmed candidate's global key from the
// resolved identity rows: a row other than the local patient's own is
// preferred, any row carrying a global key is accepted, and no
// candidate fails closed before any link.
//
// Parameters:
//   - candidates are the identity rows resolved at the gateway.
//   - ownRef is the local patient reference to prefer skipping.
//
// Returns the global key and nil, or "" and the error. Never panics.
func pickGlobalKey(candidates []contracts.IdentityCandidate, ownRef string) (string, error) {
	for _, cand := range candidates {
		if cand.LocalResourceID != ownRef && strings.TrimSpace(cand.GlobalKey) != "" {
			return cand.GlobalKey, nil
		}
	}
	for _, cand := range candidates {
		if strings.TrimSpace(cand.GlobalKey) != "" {
			return cand.GlobalKey, nil
		}
	}
	return "", errors.New("referrals: gateway reports no identity candidate")
}

// createServiceRequest materializes the local referral order for the
// confirmed patient: active order, subject = the confirmed patient, the
// incoming requester kept and the referral purpose as the note.
//
// Parameters:
//   - ctx bounds the FHIR create, task the read incoming Task,
//     patientRef the confirmed patient reference.
//
// Returns the created ServiceRequest id and nil, or "" and the write
// error. Never panics.
func (c *Confirmer) createServiceRequest(ctx context.Context, task *confirmTask,
	patientRef string,
) (string, error) {
	sr := &confirmedServiceRequest{
		ResourceType: constvars.ResourceServiceRequest,
		Status:       "active",
		Intent:       taskIntent,
		Subject:      fhir_dto.Reference{Reference: patientRef},
	}
	if task.Requester != nil {
		sr.Requester = task.Requester
	}
	if purpose := strings.TrimSpace(task.Description); purpose != "" {
		sr.Note = []fhir_dto.Annotation{{Text: purpose}}
	}
	created, err := fhir_http_client.CreateResource(ctx, c.log, c.fhir,
		c.fhirBaseURL+"/"+constvars.ResourceServiceRequest, sr,
		constvars.ResourceServiceRequest, "service_request_id")
	if err != nil || created == nil {
		if err == nil {
			err = errors.New("empty ServiceRequest response")
		}
		return "", fmt.Errorf("referrals: create service request: %w", err)
	}
	if created.ID == "" {
		return "", errors.New("referrals: created ServiceRequest carries no id")
	}
	return created.ID, nil
}
