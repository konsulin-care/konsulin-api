package referrals

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/fhir_dto"
	"konsulin-service/internal/pkg/fhir_http_client"

	"go.uber.org/zap"
)

// Referral wire constants: partner path, approval scope, the shared
// envelope service, researcher marker and the outgoing Task resource type.
const (
	referralPath     = "/api/v1/federation/referrals"
	referralScope    = "referral"
	referralService  = "referrals"
	noResearcherHash = "N/A"
	resourceTask     = "Task"

	taskStatusRequested = "requested"
	taskStatusFailed    = "failed"
	taskIntent          = "order"
)

// ErrReferralDelivery reports that the partner endpoint answered a non-2xx
// status (or the POST failed in transport) after the referral was minted
// and sealed; the outgoing Task is recorded as failed.
var ErrReferralDelivery = errors.New("referral delivery failed")

// PartnerResolver resolves one recipient party to its direct partner
// endpoint address (the T61 directory seam; *Directory satisfies it).
type PartnerResolver interface {
	// ResolvePartner returns the active Endpoint address of the partner
	// Organization carrying the recipient party identifier, or
	// ErrPartnerNotFound.
	ResolvePartner(ctx context.Context, partyID string) (string, error)
}

// TicketMinter mints the gateway approval ticket handed to the partner
// (the T62 seam; contracts.ApprovalClient satisfies it).
type TicketMinter interface {
	// Mint mints the approval ticket for the recipient party, allowlisted
	// scope and literal purpose.
	Mint(ctx context.Context, to, scope, purpose, researcherHash string) (string, error)
}

// ReferralSealer seals a referral payload to the recipient party's use:enc
// key through the SDK bound envelope (envelope.SealBound bound from the
// local party, to the recipient and to the referral service); the driver
// adapter satisfies it. A recipient without a usable use:enc key fails
// closed before any partner request is sent.
type ReferralSealer interface {
	// SealBound returns the compact JWE of data for recipientParty under
	// the named service binding.
	SealBound(ctx context.Context, recipientParty, service string, data []byte) (string, error)
}

// Sender delivers outgoing referrals (BK-REFERRAL-001): it resolves the
// partner endpoint, mints the referral ticket, seals the patient
// demographics plus ServiceRequest to the recipient's enc key, POSTs the
// sealed payload with the ticket directly to the partner, and records the
// outgoing FHIR Task.
type Sender struct {
	directory   PartnerResolver
	tickets     TicketMinter
	sealer      ReferralSealer
	fhir        *fhir_http_client.FHIRHTTPClient
	fhirBaseURL string
	httpClient  *http.Client
	log         *zap.Logger
}

// SenderDeps bundles the collaborators SendReferral needs.
type SenderDeps struct {
	// Directory resolves the recipient party's partner endpoint (required).
	Directory PartnerResolver
	// Tickets mints the referral approval ticket (required).
	Tickets TicketMinter
	// Sealer seals the payload to the recipient enc key (required).
	Sealer ReferralSealer
	// FHIRClient performs the local FHIR reads and Task writes (required).
	FHIRClient *fhir_http_client.FHIRHTTPClient
	// FHIRBaseURL is the local FHIR server root (required; trailing
	// slashes are trimmed).
	FHIRBaseURL string
	// HTTPClient posts to the partner endpoint (nil uses a 30s client).
	HTTPClient *http.Client
	// Logger records partner and Task traffic (nil uses a no-op logger).
	Logger *zap.Logger
}

// NewSender constructs an outgoing referral sender over its collaborators.
//
// Returns a Sender ready for SendReferral. Never panics.
func NewSender(deps SenderDeps) *Sender {
	log := deps.Logger
	if log == nil {
		log = zap.NewNop()
	}
	httpClient := deps.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &Sender{
		directory:   deps.Directory,
		tickets:     deps.Tickets,
		sealer:      deps.Sealer,
		fhir:        deps.FHIRClient,
		fhirBaseURL: strings.TrimRight(deps.FHIRBaseURL, "/"),
		httpClient:  httpClient,
		log:         log,
	}
}

// referralPatient is the patient demographics subset sealed to the partner:
// enough for the receiver's soft-match (name, telecom, birth date), never
// the full local record.
type referralPatient struct {
	// ID is the local Patient id, carried for traceability.
	ID string `json:"id,omitempty"`
	// Identifier carries the patient identifiers used for matching.
	Identifier []fhir_dto.Identifier `json:"identifier,omitempty"`
	// Name carries the patient names the receiver soft-matches on.
	Name []fhir_dto.HumanName `json:"name,omitempty"`
	// Telecom carries email and phone contact points for soft-matching.
	Telecom []fhir_dto.ContactPoint `json:"telecom,omitempty"`
	// Gender is the administrative gender.
	Gender string `json:"gender,omitempty"`
	// BirthDate is the patient birth date.
	BirthDate string `json:"birthDate,omitempty"`
}

// referralPayload is the plaintext sealed into the bound envelope.
type referralPayload struct {
	// Patient is the sealed demographics subset.
	Patient referralPatient `json:"patient"`
	// ServiceRequest is the referral order sent verbatim.
	ServiceRequest json.RawMessage `json:"serviceRequest"`
}

// outgoingTask is the FHIR Task recorded for an outgoing referral.
type outgoingTask struct {
	// ID is the created Task id echoed by the FHIR server.
	ID string `json:"id,omitempty"`
	// ResourceType is always "Task".
	ResourceType string `json:"resourceType"`
	// Status is "requested" on a 2xx partner answer, "failed" otherwise.
	Status string `json:"status"`
	// Intent is always "order".
	Intent string `json:"intent"`
	// Description carries the referral purpose.
	Description string `json:"description,omitempty"`
	// Requester is the caller's FHIR reference.
	Requester *fhir_dto.Reference `json:"requester,omitempty"`
	// For is the referred patient reference.
	For fhir_dto.Reference `json:"for,omitempty"`
	// Focus is the referenced ServiceRequest.
	Focus *fhir_dto.Reference `json:"focus,omitempty"`
}

// patientDemographics projects a local Patient onto the sealed subset.
//
// Returns the demographics members the receiver soft-matches on. Never panics.
func patientDemographics(p *fhir_dto.Patient) referralPatient {
	if p == nil {
		return referralPatient{}
	}
	return referralPatient{
		ID:         p.ID,
		Identifier: p.Identifier,
		Name:       p.Name,
		Telecom:    p.Telecom,
		Gender:     p.Gender,
		BirthDate:  p.BirthDate,
	}
}

// SendReferral sends one referral to the recipient party (BK-REFERRAL-001):
// it resolves the partner endpoint (T61), fetches the patient demographics,
// mints the referral ticket (T62), seals {patient demographics,
// ServiceRequest} to the recipient enc key, and POSTs the sealed payload to
// <endpoint>/api/v1/federation/referrals with the ticket as a bearer
// Authorization header. The outgoing FHIR Task is recorded as "requested"
// on a 2xx answer and as "failed" when the partner answers non-2xx (or the
// POST fails in transport); a record failure after delivery is logged, not
// returned, because a resend would replay a consumed ticket.
//
// Parameters:
//   - ctx bounds every downstream call.
//   - caller is the requester's FHIR reference on the outgoing Task.
//   - patientID is the local Patient whose demographics are sealed.
//   - recipientParty is the recipient party identifier.
//   - serviceRequest is the ServiceRequest resource sent verbatim.
//   - purpose is the literal handoff reason minted into the ticket.
//
// Returns the recorded outgoing Task id and nil on a 2xx partner answer,
// or "" and the error — ErrPartnerNotFound, a configuration error, or
// ErrReferralDelivery wrapping the partner status. Never panics.
//
//nolint:gocyclo // TODO: reduce complexity below 15 (follow-up)
func (s *Sender) SendReferral(ctx context.Context, caller, patientID, recipientParty string,
	serviceRequest json.RawMessage, purpose string,
) (string, error) {
	if s == nil || s.directory == nil || s.tickets == nil || s.sealer == nil ||
		s.fhir == nil || s.fhirBaseURL == "" {
		return "", errors.New("referrals: sender not configured")
	}
	recipientParty = strings.TrimSpace(recipientParty)
	purpose = strings.TrimSpace(purpose)
	if recipientParty == "" || strings.TrimSpace(patientID) == "" || purpose == "" {
		return "", errors.New("referrals: missing patient, recipient or purpose")
	}
	if !json.Valid(serviceRequest) {
		return "", errors.New("referrals: service request is not valid JSON")
	}

	endpoint, err := s.directory.ResolvePartner(ctx, recipientParty)
	if err != nil {
		return "", err
	}
	patient, err := fhir_http_client.GetResource[fhir_dto.Patient](ctx, s.log, s.fhir,
		s.fhirBaseURL+"/"+constvars.ResourcePatient, patientID, constvars.ResourcePatient, "patient_id")
	if err != nil {
		return "", fmt.Errorf("referrals: read patient: %w", err)
	}
	data, err := json.Marshal(referralPayload{
		Patient:        patientDemographics(patient),
		ServiceRequest: serviceRequest,
	})
	if err != nil {
		return "", fmt.Errorf("referrals: build payload: %w", err)
	}

	ticket, err := s.tickets.Mint(ctx, recipientParty, referralScope, purpose, noResearcherHash)
	if err != nil {
		return "", fmt.Errorf("referrals: mint ticket: %w", err)
	}
	sealed, err := s.sealer.SealBound(ctx, recipientParty, referralService, data)
	if err != nil {
		return "", fmt.Errorf("referrals: seal payload: %w", err)
	}

	status, err := s.post(ctx, endpoint, ticket, sealed)
	if err != nil {
		s.record(ctx, taskStatusFailed, caller, patientID, purpose, serviceRequest)
		return "", fmt.Errorf("%w: %w", ErrReferralDelivery, err)
	}
	if status < http.StatusOK || status >= http.StatusMultipleChoices {
		s.record(ctx, taskStatusFailed, caller, patientID, purpose, serviceRequest)
		return "", fmt.Errorf("%w: partner answered status %d", ErrReferralDelivery, status)
	}
	taskID := s.record(ctx, taskStatusRequested, caller, patientID, purpose, serviceRequest)
	return taskID, nil
}

// post sends the sealed payload with the bearer ticket to the partner
// endpoint. Returns the partner status code, or the transport error.
func (s *Sender) post(ctx context.Context, endpoint, ticket, sealed string) (int, error) {
	body, err := json.Marshal(map[string]string{"payload": sealed})
	if err != nil {
		return 0, fmt.Errorf("encode referral body: %w", err)
	}
	target := strings.TrimRight(endpoint, "/") + referralPath
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(body))
	if err != nil {
		return 0, fmt.Errorf("build referral request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+ticket)
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("post referral: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode, nil
}

// record best-effort creates the outgoing FHIR Task and returns its id
// ("" when the write fails; the failure is logged so a delivered referral
// is never failed by its audit trail).
func (s *Sender) record(ctx context.Context, status, caller, patientID, purpose string,
	serviceRequest json.RawMessage,
) string {
	task := &outgoingTask{
		ResourceType: resourceTask,
		Status:       status,
		Intent:       taskIntent,
		Description:  purpose,
		For:          fhir_dto.Reference{Reference: constvars.FHIRRefPrefixPatient + patientID},
	}
	if caller = strings.TrimSpace(caller); caller != "" {
		task.Requester = &fhir_dto.Reference{Reference: caller}
	}
	var focus struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(serviceRequest, &focus); err == nil && focus.ID != "" {
		task.Focus = &fhir_dto.Reference{Reference: constvars.ResourceServiceRequest + "/" + focus.ID}
	}
	created, err := fhir_http_client.CreateResource(ctx, s.log, s.fhir,
		s.fhirBaseURL+"/"+resourceTask, task, resourceTask, "task_id")
	if err != nil || created == nil {
		s.log.Error("referrals: record outgoing task failed", zap.Error(err))
		return ""
	}
	return created.ID
}
