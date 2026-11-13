package referrals

import (
	"context"
	"crypto/ecdh"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"konsulin-service/internal/app/services/core/federation"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/fhir_dto"
	"konsulin-service/internal/pkg/fhir_http_client"
	"konsulin-service/internal/pkg/utils"

	"github.com/konsulin-care/gateway-sdk-go/envelope"
	"github.com/konsulin-care/gateway-sdk-go/identity"
	"go.uber.org/zap"
)

// inbound wire constants: the ticket claims read after verification, the
// Task.input type labeling one soft-match candidate and the FHIR Patient
// search parameters the soft-match runs.
const (
	claimSub      = "sub"
	claimPurpose  = "purpose"
	candidateType = "candidate-patient"
	paramEmail    = "email"
	paramPhone    = "phone"
	paramName     = "name"
	paramBirth    = "birthdate"
	emailSystem   = "email"
	phoneSystem   = "phone"
)

// ErrReferralRejected reports that an incoming referral was refused before
// anything was stored: an unusable ticket, an envelope that does not open
// under the party's key or a payload that does not decode.
var ErrReferralRejected = errors.New("referral rejected")

// TicketVerifier checks one directly presented approval ticket for the
// expected scope; contracts.ApprovalClient satisfies it (the T62 seam).
type TicketVerifier interface {
	// VerifyTicket returns nil once the ticket verifies for expectedScope
	// (signature, audience, expiry, scope and single-use jti), or the error
	// carrying the SDK sentinel.
	VerifyTicket(ctx context.Context, token, expectedScope string) error
}

// Receiver accepts incoming referrals (BK-REFERRAL-002): it verifies the
// presented ticket for the referral scope, opens the bound envelope under
// the party's own use:enc key, soft-matches local Patients by peppered
// email/phone hash or by name plus birthDate, and stores one incoming FHIR
// Task in status "requested" listing the candidate patients. It never links
// a patient — the human confirmation step owns that decision.
type Receiver struct {
	tickets     TicketVerifier
	encKey      *ecdh.PrivateKey
	partyID     string
	fhir        *fhir_http_client.FHIRHTTPClient
	fhirBaseURL string
	log         *zap.Logger
}

// ReceiverDeps bundles the collaborators ReceiveReferral needs.
type ReceiverDeps struct {
	// Tickets verifies the presented approval ticket (required; the gateway
	// adapter's VerifyTicket satisfies it).
	Tickets TicketVerifier
	// EncPrivateKey is the party's own use:enc key used to open the bound
	// envelope (nil reads APP_PARTY_ENC_PRIVATE_KEY at call time).
	EncPrivateKey *ecdh.PrivateKey
	// PartyID is this party's own id, the envelope's expected "to"
	// ("" reads APP_PARTY_ID at call time).
	PartyID string
	// FHIRClient performs the local Patient reads and Task write (required).
	FHIRClient *fhir_http_client.FHIRHTTPClient
	// FHIRBaseURL is the local FHIR server root (required; trailing
	// slashes are trimmed).
	FHIRBaseURL string
	// Logger records matching and Task traffic (nil uses a no-op logger).
	Logger *zap.Logger
}

// NewReceiver constructs an incoming referral receiver over its
// collaborators.
//
// Returns a Receiver ready for ReceiveReferral. Never panics.
func NewReceiver(deps ReceiverDeps) *Receiver {
	log := deps.Logger
	if log == nil {
		log = zap.NewNop()
	}
	return &Receiver{
		tickets:     deps.Tickets,
		encKey:      deps.EncPrivateKey,
		partyID:     strings.TrimSpace(deps.PartyID),
		fhir:        deps.FHIRClient,
		fhirBaseURL: strings.TrimRight(deps.FHIRBaseURL, "/"),
		log:         log,
	}
}

// ticketClaims is the verified ticket payload read after VerifyTicket: the
// sending party (the envelope's expected "from") and the literal purpose.
type ticketClaims struct {
	Sub     string `json:"sub"`
	Purpose string `json:"purpose"`
}

// incomingTaskInput is one soft-match candidate listed on the incoming Task.
type incomingTaskInput struct {
	// Type labels the entry as a soft-match candidate.
	Type struct {
		Text string `json:"text"`
	} `json:"type"`
	// ValueReference points at the candidate Patient.
	ValueReference fhir_dto.Reference `json:"valueReference"`
}

// incomingTask is the FHIR Task stored for an incoming referral.
type incomingTask struct {
	// ID is the created Task id echoed by the FHIR server.
	ID string `json:"id,omitempty"`
	// ResourceType is always "Task".
	ResourceType string `json:"resourceType"`
	// Status is always "requested": a human confirms before any link.
	Status string `json:"status"`
	// Intent is always "order".
	Intent string `json:"intent"`
	// Description carries the ticket's purpose when present.
	Description string `json:"description,omitempty"`
	// Focus references the referred ServiceRequest.
	Focus *fhir_dto.Reference `json:"focus,omitempty"`
	// Input lists the soft-matched candidate patients; never a link.
	Input []incomingTaskInput `json:"input,omitempty"`
}

// claimsOf decodes the verified ticket's payload segment. The signature has
// already been proven by VerifyTicket, so the plaintext claims are trusted
// at this point.
//
// Returns the claim set and nil, or an error when the segment is not
// decodable JSON. Never panics.
func claimsOf(ticket string) (ticketClaims, error) {
	parts := strings.Split(ticket, ".")
	if len(parts) != 3 {
		return ticketClaims{}, errors.New("ticket is not a compact JWS")
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return ticketClaims{}, fmt.Errorf("ticket payload decode: %w", err)
	}
	var claims ticketClaims
	if err := json.Unmarshal(raw, &claims); err != nil {
		return ticketClaims{}, fmt.Errorf("ticket payload parse: %w", err)
	}
	return claims, nil
}

// ownKey resolves the receiver's use:enc private key: the configured key
// first, then APP_PARTY_ENC_PRIVATE_KEY. The key value never appears in an
// error.
//
// Returns the private key and nil, or an error naming only the variable.
func (r *Receiver) ownKey() (*ecdh.PrivateKey, error) {
	if r.encKey != nil {
		return r.encKey, nil
	}
	raw := strings.TrimSpace(utils.GetEnvString("APP_PARTY_ENC_PRIVATE_KEY", ""))
	if raw == "" {
		return nil, errors.New("referrals: APP_PARTY_ENC_PRIVATE_KEY is empty")
	}
	key, err := envelope.ParseX25519PrivateKey(raw)
	if err != nil {
		return nil, errors.New("referrals: APP_PARTY_ENC_PRIVATE_KEY is unparseable")
	}
	return key, nil
}

// ownPartyID resolves this party's id: the configured value first, then
// APP_PARTY_ID. Returns the id, or an error when neither is set.
func (r *Receiver) ownPartyID() (string, error) {
	if r.partyID != "" {
		return r.partyID, nil
	}
	self := strings.TrimSpace(utils.GetEnvString("APP_PARTY_ID", ""))
	if self == "" {
		return "", errors.New("referrals: APP_PARTY_ID is empty")
	}
	return self, nil
}

// ReceiveReferral accepts one inbound referral (BK-REFERRAL-002): it
// verifies the presented ticket for the referral scope (single-use jti, so
// a replay is denied before anything is read or written), opens the bound
// envelope under the party's own use:enc key with expectations
// from = ticket sub, to = this party, service = "referrals", soft-matches
// local Patients by peppered email/phone hash or name plus birthDate, and
// stores one incoming Task in status "requested" listing the candidates.
// No patient is ever linked automatically — a rejected ticket or envelope
// stores nothing.
//
// Parameters:
//   - ctx bounds the ticket check, the envelope open and the FHIR calls.
//   - ticket is the compact ks-approval+jwt presented by the sender.
//   - sealed is the compact bound JWE carrying {patient, serviceRequest}.
//
// Returns the stored incoming Task id and nil, or "" and the error —
// ErrReferralRejected wrapping the ticket, envelope or payload failure, or
// a configuration/FHIR error. Never panics.
func (r *Receiver) ReceiveReferral(ctx context.Context, ticket, sealed string) (string, error) {
	if r == nil || r.tickets == nil || r.fhir == nil || r.fhirBaseURL == "" {
		return "", errors.New("referrals: receiver not configured")
	}
	ticket = strings.TrimSpace(ticket)
	sealed = strings.TrimSpace(sealed)
	if ticket == "" || sealed == "" {
		return "", fmt.Errorf("%w: missing ticket or envelope", ErrReferralRejected)
	}

	if err := r.tickets.VerifyTicket(ctx, ticket, referralScope); err != nil {
		return "", fmt.Errorf("%w: verify ticket: %w", ErrReferralRejected, err)
	}
	claims, err := claimsOf(ticket)
	if err != nil {
		return "", fmt.Errorf("%w: ticket claims: %w", ErrReferralRejected, err)
	}
	if strings.TrimSpace(claims.Sub) == "" {
		return "", fmt.Errorf("%w: ticket carries no sender party", ErrReferralRejected)
	}
	key, err := r.ownKey()
	if err != nil {
		return "", err
	}
	self, err := r.ownPartyID()
	if err != nil {
		return "", err
	}
	opened, err := envelope.OpenBound(ctx, sealed, key, envelope.Expectations{
		From: claims.Sub, To: self, Service: referralService,
	}, nil, time.Now)
	if err != nil {
		return "", fmt.Errorf("%w: open envelope: %w", ErrReferralRejected, err)
	}
	var payload referralPayload
	if err := json.Unmarshal(opened, &payload); err != nil {
		return "", fmt.Errorf("%w: payload decode: %w", ErrReferralRejected, err)
	}

	candidates := r.softMatch(ctx, payload.Patient)
	return r.record(ctx, claims.Purpose, payload.ServiceRequest, candidates)
}

// softMatch collects the local Patient candidates for the incoming
// demographics: the peppered email/phone hash strategy (when a pepper is
// available) and the name plus birthDate strategy, unioned and deduped by
// Patient id. A strategy that fails is logged and skipped — soft matching
// only surfaces candidates, it never links.
//
// Parameters:
//   - ctx bounds the FHIR searches.
//   - patient is the sealed incoming demographics.
//
// Returns the deduped candidate Patient references (empty when nothing
// matches). Never panics.
func (r *Receiver) softMatch(ctx context.Context, patient referralPatient) []fhir_dto.Reference {
	seen := make(map[string]bool)
	var out []fhir_dto.Reference
	add := func(id string) {
		if id == "" || seen[id] {
			return
		}
		seen[id] = true
		out = append(out, fhir_dto.Reference{Reference: constvars.FHIRRefPrefixPatient + id})
	}

	email, phone := referralContact(patient.Telecom)
	r.matchByHash(ctx, email, phone, add)
	r.matchByNameBirth(ctx, patient, add)
	return out
}

// referralContact extracts the first email and phone value from the sealed
// telecom list. Returns the two values ("" when absent). Never panics.
func referralContact(points []fhir_dto.ContactPoint) (email, phone string) {
	for _, cp := range points {
		value := strings.TrimSpace(cp.Value)
		if value == "" {
			continue
		}
		switch strings.ToLower(string(cp.System)) {
		case emailSystem:
			if email == "" {
				email = value
			}
		case phoneSystem:
			if phone == "" {
				phone = value
			}
		}
	}
	return email, phone
}

// matchByHash soft-matches by peppered email/phone hash: local Patients are
// searched by the incoming contact values, then each hit is accepted only
// when its own peppered hash equals the incoming peppered hash (normalized
// comparison, never plaintext equality). A missing pepper or a failed
// search is logged and skips the strategy.
//
// Parameters:
//   - ctx bounds the FHIR searches, email and phone the incoming contacts,
//     add records one accepted candidate id.
//
// Returns nothing. Never panics.
func (r *Receiver) matchByHash(ctx context.Context, email, phone string,
	add func(string),
) {
	if email == "" && phone == "" {
		return
	}
	pepper, err := federation.LoadPepper()
	if err != nil {
		r.log.Warn("referrals: hash soft-match skipped: no pepper", zap.Error(err))
		return
	}
	incoming, err := incomingHashes(email, phone, pepper)
	if err != nil {
		r.log.Warn("referrals: hash soft-match skipped: incoming contact", zap.Error(err))
		return
	}
	r.hashContact(ctx, paramEmail, email, incoming, pepper, add)
	r.hashContact(ctx, paramPhone, phone, incoming, pepper, add)
}

// hashPair holds the peppered hashes of the incoming contacts ("" for the
// side that is absent).
type hashPair struct {
	email string
	phone string
}

// incomingHashes computes the peppered hashes of the incoming contacts.
// A contact that is absent is skipped, never hashed.
//
// Returns the hash pair and nil, or the identity hashing error.
// Never panics.
func incomingHashes(email, phone string, pepper []byte) (hashPair, error) {
	var out hashPair
	if email != "" {
		hash, err := identity.HashEmail(email, pepper)
		if err != nil {
			return hashPair{}, err
		}
		out.email = hash
	}
	if phone != "" {
		hash, err := identity.HashPhone(phone, pepper)
		if err != nil {
			return hashPair{}, err
		}
		out.phone = hash
	}
	return out, nil
}

// hashContact searches local Patients by one incoming contact value and
// accepts the hits whose own peppered hash equals the incoming peppered
// hash (never plaintext equality). A failed search is logged and skips
// this contact.
//
// Parameters:
//   - ctx bounds the FHIR search, param/value the contact search,
//     incoming the incoming hashes, pepper the federation pepper,
//     add records one accepted candidate id.
//
// Returns nothing. Never panics.
func (r *Receiver) hashContact(ctx context.Context, param, value string,
	incoming hashPair, pepper []byte, add func(string),
) {
	if value == "" {
		return
	}
	hits, err := r.searchPatients(ctx, param, value)
	if err != nil {
		r.log.Warn("referrals: hash soft-match search failed", zap.Error(err))
		return
	}
	for _, local := range hits {
		localEmail, localPhone := referralContact(local.Telecom)
		if hashMatches(localEmail, incoming.email, pepper, identity.HashEmail) ||
			hashMatches(localPhone, incoming.phone, pepper, identity.HashPhone) {
			add(local.ID)
		}
	}
}

// hashMatches reports whether the local value is present and its peppered
// hash equals the incoming peppered hash. Never panics.
func hashMatches(localValue, incomingHash string, pepper []byte,
	hash func(string, []byte) (string, error),
) bool {
	if localValue == "" || incomingHash == "" || incomingHash == identity.NAMarker {
		return false
	}
	got, err := hash(localValue, pepper)
	if err != nil {
		return false
	}
	return identity.Equal(got, incomingHash)
}

// matchByNameBirth soft-matches by name plus birthDate: local Patients are
// searched by the incoming name token and birthDate, then each hit is
// accepted only when the birthDate is equal and a family or given name
// matches case-insensitively. A failed search is logged and skips the
// strategy.
//
// Parameters:
//   - ctx bounds the FHIR search, patient the incoming demographics,
//     add records one accepted candidate id.
//
// Returns nothing. Never panics.
func (r *Receiver) matchByNameBirth(ctx context.Context, patient referralPatient,
	add func(string),
) {
	birth := strings.TrimSpace(patient.BirthDate)
	token := nameToken(patient.Name)
	if birth == "" || token == "" {
		return
	}
	query := paramName + "=" + url.QueryEscape(token) + "&" +
		paramBirth + "=" + url.QueryEscape(birth)
	hits, err := r.searchPatients(ctx, query, "")
	if err != nil {
		r.log.Warn("referrals: name/birth soft-match search failed", zap.Error(err))
		return
	}
	for _, local := range hits {
		if strings.TrimSpace(local.BirthDate) == birth && namesOverlap(patient.Name, local.Name) {
			add(local.ID)
		}
	}
}

// nameToken picks the search token for the incoming names: the family, the
// first given, or the first free-text name. Returns "" when the list is
// empty. Never panics.
func nameToken(names []fhir_dto.HumanName) string {
	for _, n := range names {
		if f := strings.TrimSpace(n.Family); f != "" {
			return f
		}
		for _, g := range n.Given {
			if g = strings.TrimSpace(g); g != "" {
				return g
			}
		}
		if t := strings.TrimSpace(n.Text); t != "" {
			return t
		}
	}
	return ""
}

// namesOverlap reports whether any family or given name of the incoming
// names equals a local name member case-insensitively. Never panics.
func namesOverlap(incoming, local []fhir_dto.HumanName) bool {
	for _, in := range incoming {
		if nameOverlapsLocal(in, local) {
			return true
		}
	}
	return false
}

// nameOverlapsLocal reports whether one incoming name overlaps the local
// names: its family equals the first local family, or one of its given
// names equals a local given or family name. Never panics.
func nameOverlapsLocal(in fhir_dto.HumanName, local []fhir_dto.HumanName) bool {
	family := strings.TrimSpace(in.Family)
	if family != "" && strings.EqualFold(family, localFamily(local)) {
		return true
	}
	for _, given := range in.Given {
		if givenMatchesLocal(strings.TrimSpace(given), local) {
			return true
		}
	}
	return false
}

// givenMatchesLocal reports whether the incoming given-name token matches
// any local given or family name (an empty token never matches).
// Never panics.
func givenMatchesLocal(given string, local []fhir_dto.HumanName) bool {
	if given == "" {
		return false
	}
	for _, ln := range local {
		if containsFold(ln.Given, given) ||
			strings.EqualFold(given, strings.TrimSpace(ln.Family)) {
			return true
		}
	}
	return false
}

// containsFold reports whether any list member equals want
// case-insensitively after trimming spaces. Never panics.
func containsFold(values []string, want string) bool {
	for _, v := range values {
		if strings.EqualFold(strings.TrimSpace(v), want) {
			return true
		}
	}
	return false
}

// localFamily returns the first non-empty local family name ("" when the
// list holds none). Never panics.
func localFamily(names []fhir_dto.HumanName) string {
	for _, n := range names {
		if f := strings.TrimSpace(n.Family); f != "" {
			return f
		}
	}
	return ""
}

// searchPatients runs one Patient search for the given query. The query is
// either "param=value" (already escaped) or "param" with the value passed
// separately.
//
// Parameters:
//   - ctx bounds the search, param the query string, value the parameter
//     value appended when param carries no '='.
//
// Returns the matched patients and nil, or the search error.
func (r *Receiver) searchPatients(ctx context.Context, param, value string) ([]fhir_dto.Patient, error) {
	query := param
	if !strings.Contains(param, "=") {
		query = param + "=" + url.QueryEscape(value)
	}
	searchURL := fmt.Sprintf("%s/%s?%s", r.fhirBaseURL, constvars.ResourcePatient, query)
	return fhir_http_client.SearchResources[fhir_dto.Patient](ctx, r.log, r.fhir,
		searchURL, constvars.ResourcePatient)
}

// record stores the incoming FHIR Task in status "requested": it focuses
// the referred ServiceRequest and lists one Task.input entry per soft-match
// candidate — a candidate listing, never a Patient link.
//
// Parameters:
//   - ctx bounds the Task write, purpose the ticket's literal purpose,
//     serviceRequest the referred order, candidates the soft-matched
//     patients.
//
// Returns the created Task id and nil, or "" and the write error.
// Never panics.
func (r *Receiver) record(ctx context.Context, purpose string,
	serviceRequest json.RawMessage, candidates []fhir_dto.Reference,
) (string, error) {
	task := &incomingTask{
		ResourceType: resourceTask,
		Status:       taskStatusRequested,
		Intent:       taskIntent,
		Description:  strings.TrimSpace(purpose),
	}
	var focus struct {
		ID string `json:"id"`
	}
	if json.Valid(serviceRequest) && json.Unmarshal(serviceRequest, &focus) == nil && focus.ID != "" {
		task.Focus = &fhir_dto.Reference{Reference: constvars.ResourceServiceRequest + "/" + focus.ID}
	}
	for _, candidate := range candidates {
		entry := incomingTaskInput{ValueReference: candidate}
		entry.Type.Text = candidateType
		task.Input = append(task.Input, entry)
	}
	created, err := fhir_http_client.CreateResource(ctx, r.log, r.fhir,
		r.fhirBaseURL+"/"+resourceTask, task, resourceTask, "task_id")
	if err != nil || created == nil {
		if err == nil {
			err = errors.New("empty Task response")
		}
		return "", fmt.Errorf("referrals: record incoming task: %w", err)
	}
	return created.ID, nil
}
