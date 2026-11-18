package referrals

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/konsulin-care/konsulin-api/internal/pkg/constvars"
	"github.com/konsulin-care/konsulin-api/internal/pkg/fhir_dto"
	"github.com/konsulin-care/konsulin-api/internal/pkg/fhir_http_client"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// Fixtures for the fake collaborators.
const (
	ticketFixture   = "ticket-abc"
	sealedFixture   = "sealed-jwe-compact"
	callerFixture   = "Practitioner/prac-1"
	purposeFixture  = "cardiology-handoff"
	serviceReqFacts = `{"resourceType":"ServiceRequest","id":"sr-1","status":"active","intent":"order"}`
)

// fakeMinter records the Mint arguments and serves the fixed ticket.
type fakeMinter struct {
	// to is the recipient party of the last call.
	to string
	// scope is the allowlisted scope of the last call.
	scope string
	// purpose is the literal purpose of the last call.
	purpose string
	// hash is the researcher hash of the last call.
	hash string
}

// Mint records the arguments and returns the fixed ticket.
func (f *fakeMinter) Mint(_ context.Context, to, scope, purpose, researcherHash string) (string, error) {
	f.to = to
	f.scope = scope
	f.purpose = purpose
	f.hash = researcherHash
	return ticketFixture, nil
}

// fakeSealer records the SealBound arguments and serves the fixed JWE.
type fakeSealer struct {
	// to is the recipient party of the last call.
	to string
	// service is the binding service of the last call.
	service string
	// data is the plaintext payload of the last call.
	data []byte
}

// SealBound records the arguments and returns the fixed compact JWE.
func (f *fakeSealer) SealBound(_ context.Context, recipientParty, service string, data []byte) (string, error) {
	f.to = recipientParty
	f.service = service
	f.data = data
	return sealedFixture, nil
}

// sendFixture wires a Sender against fake partner and FHIR servers.
type sendFixture struct {
	// sender is the unit under test.
	sender *Sender
	// minter is the fake ticket minter.
	minter *fakeMinter
	// sealer is the fake envelope sealer.
	sealer *fakeSealer
	// partnerPath is the path the partner endpoint received.
	partnerPath string
	// partnerAuth is the Authorization header the partner received.
	partnerAuth string
	// partnerBody is the raw body the partner received.
	partnerBody []byte
	// taskBody is the raw body the FHIR server received at POST /Task.
	taskBody []byte
}

// newSendFixture builds the sender over the real T61 Directory, a fake
// minter and sealer, an httptest FHIR server (Organization, Endpoint and
// Patient reads plus the Task write) and an httptest partner endpoint
// answering partnerStatus at /api/v1/federation/referrals.
func newSendFixture(t *testing.T, partnerStatus int) *sendFixture {
	t.Helper()
	fx := &sendFixture{minter: &fakeMinter{}, sealer: &fakeSealer{}}

	partner := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fx.partnerPath = r.URL.Path
		fx.partnerAuth = r.Header.Get("Authorization")
		fx.partnerBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(partnerStatus)
	}))
	t.Cleanup(partner.Close)

	org := partnerOrg("org-1", "party-123")
	ep := endpointResource{}
	ep.ResourceType = resourceEndpoint
	ep.ID = "ep-1"
	ep.Status = endpointStatusActive
	ep.Address = partner.URL
	ep.ManagingOrganization = fhir_dto.Reference{Reference: "Organization/org-1"}

	patient := fhir_dto.Patient{ID: "pat-1", ResourceType: constvars.ResourcePatient}
	patient.Name = []fhir_dto.HumanName{{Use: "official", Family: "Lovelace", Given: []string{"Ada"}}}
	patient.Gender = "female"
	patient.BirthDate = "1990-12-10"

	fhir := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/"+resourceTask:
			fx.taskBody, _ = io.ReadAll(r.Body)
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"resourceType":"Task","id":"task-out-1"}`))
		case r.URL.Path == "/"+constvars.ResourceOrganization:
			writeBundle(t, w, partnerBundle(mustMarshalEntry(t, org)))
		case r.URL.Path == "/"+resourceEndpoint:
			writeBundle(t, w, partnerBundle(mustMarshalEntry(t, ep)))
		case r.URL.Path == "/"+constvars.ResourcePatient+"/pat-1":
			w.Header().Set("Content-Type", "application/fhir+json")
			_ = json.NewEncoder(w).Encode(patient)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(fhir.Close)

	deps := SenderDeps{}
	deps.Directory = NewDirectory(fhir_http_client.New(zap.NewNop()), fhir.URL, zap.NewNop())
	deps.Tickets = fx.minter
	deps.Sealer = fx.sealer
	deps.FHIRClient = fhir_http_client.New(zap.NewNop())
	deps.FHIRBaseURL = fhir.URL
	fx.sender = NewSender(deps)
	return fx
}

// TestSendReferral_RejectsInvalidInput covers expected output 1: blank
// patient, recipient or purpose, or a ServiceRequest that is not valid JSON,
// fails before the partner is resolved or any request is posted.
func TestSendReferral_RejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name       string
		patientID  string
		recipient  string
		purpose    string
		serviceReq string
		wantErr    string
	}{
		{name: "blank patient", patientID: "  ", recipient: "party-123", purpose: purposeFixture, serviceReq: serviceReqFacts, wantErr: "missing patient, recipient or purpose"},
		{name: "blank recipient", patientID: "pat-1", recipient: "  ", purpose: purposeFixture, serviceReq: serviceReqFacts, wantErr: "missing patient, recipient or purpose"},
		{name: "blank purpose", patientID: "pat-1", recipient: "party-123", purpose: "  ", serviceReq: serviceReqFacts, wantErr: "missing patient, recipient or purpose"},
		{name: "invalid json", patientID: "pat-1", recipient: "party-123", purpose: purposeFixture, serviceReq: "{not-json", wantErr: "service request is not valid JSON"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fx := newSendFixture(t, http.StatusOK)
			taskID, err := fx.sender.SendReferral(context.Background(), callerFixture, tc.patientID,
				tc.recipient, json.RawMessage(tc.serviceReq), tc.purpose)

			require.Error(t, err, "invalid input must fail")
			assert.Contains(t, err.Error(), tc.wantErr, "the validation error must be preserved")
			assert.Empty(t, taskID, "no Task id must be returned")
			assert.Empty(t, fx.partnerPath, "the partner must not be contacted")
			assert.Empty(t, fx.minter.to, "no ticket must be minted")
			assert.Empty(t, fx.sealer.to, "no payload must be sealed")
		})
	}
}

// TestSendReferral_UnconfiguredSenderRejects covers the configuration guard:
// a sender missing its collaborators fails before validating input.
func TestSendReferral_UnconfiguredSenderRejects(t *testing.T) {
	taskID, err := (&Sender{}).SendReferral(context.Background(), callerFixture, "pat-1",
		"party-123", json.RawMessage(serviceReqFacts), purposeFixture)
	require.Error(t, err, "an unconfigured sender must fail")
	assert.Contains(t, err.Error(), "sender not configured")
	assert.Empty(t, taskID)
}

// TestSendReferral_SealsAndPostsTicketToPartnerEndpoint covers expected
// output 1: the referral is sealed to the recipient and posted with the
// ticket to the partner endpoint.
func TestSendReferral_SealsAndPostsTicketToPartnerEndpoint(t *testing.T) {
	fx := newSendFixture(t, http.StatusOK)
	taskID, err := fx.sender.SendReferral(context.Background(), callerFixture, "pat-1",
		"party-123", json.RawMessage(serviceReqFacts), purposeFixture)

	require.NoError(t, err, "a 2xx partner answer must succeed")
	assert.Equal(t, "task-out-1", taskID, "the recorded Task id must be returned")

	// The partner endpoint received the POST with the minted ticket.
	assert.Equal(t, referralPath, fx.partnerPath, "must post to the partner endpoint path")
	assert.Equal(t, "Bearer "+ticketFixture, fx.partnerAuth, "must carry the ticket as bearer")
	var posted struct {
		Payload string `json:"payload"`
	}
	require.NoError(t, json.Unmarshal(fx.partnerBody, &posted))
	assert.Equal(t, sealedFixture, posted.Payload, "the body must carry the sealed payload")

	// The payload was sealed to the recipient with the right binding.
	assert.Equal(t, "party-123", fx.sealer.to, "must seal to the recipient party")
	assert.Equal(t, referralService, fx.sealer.service, "must bind the referral service")
	assert.Contains(t, string(fx.sealer.data), "Lovelace", "patient demographics must be sealed")
	assert.Contains(t, string(fx.sealer.data), "1990-12-10", "birth date must be sealed")
	assert.Contains(t, string(fx.sealer.data), "sr-1", "the ServiceRequest must be sealed")

	// The ticket was minted for the recipient with the referral scope.
	assert.Equal(t, "party-123", fx.minter.to, "must mint for the recipient party")
	assert.Equal(t, referralScope, fx.minter.scope, "must mint with the referral scope")
	assert.Equal(t, purposeFixture, fx.minter.purpose, "must mint the literal purpose")
	assert.Equal(t, noResearcherHash, fx.minter.hash, "no researcher applies to a handoff")
}

// TestSendReferral_Non2xxRecordsFailedTask covers the failure half of
// expected output 2: the outgoing Task is failed when the partner answers
// non-2xx.
func TestSendReferral_Non2xxRecordsFailedTask(t *testing.T) {
	fx := newSendFixture(t, http.StatusBadGateway)
	taskID, err := fx.sender.SendReferral(context.Background(), callerFixture, "pat-1",
		"party-123", json.RawMessage(serviceReqFacts), purposeFixture)

	require.Error(t, err, "a non-2xx partner answer must fail")
	assert.ErrorIs(t, err, ErrReferralDelivery, "the failure must carry the delivery sentinel")
	assert.Empty(t, taskID, "no success Task id must be returned")

	var task map[string]any
	require.NoError(t, json.Unmarshal(fx.taskBody, &task), "the outgoing Task must be recorded")
	assert.Equal(t, resourceTask, task["resourceType"], "the record must be a Task")
	assert.Equal(t, taskStatusFailed, task["status"], "a non-2xx answer must record failed")
	assert.Equal(t, taskIntent, task["intent"], "the Task must stay an order")
}

// TestSendReferral_SuccessRecordsRequestedTask covers the success half of
// expected output 2: the outgoing Task is recorded as requested on success.
func TestSendReferral_SuccessRecordsRequestedTask(t *testing.T) {
	fx := newSendFixture(t, http.StatusCreated)
	_, err := fx.sender.SendReferral(context.Background(), callerFixture, "pat-1",
		"party-123", json.RawMessage(serviceReqFacts), purposeFixture)
	require.NoError(t, err)

	var task map[string]any
	require.NoError(t, json.Unmarshal(fx.taskBody, &task), "the outgoing Task must be recorded")
	assert.Equal(t, taskStatusRequested, task["status"], "a 2xx answer must record requested")
	assert.Equal(t, purposeFixture, task["description"], "the purpose must be recorded")
	focus, _ := task["focus"].(map[string]any)
	assert.Equal(t, "ServiceRequest/sr-1", focus["reference"], "the focus must reference the order")
	requester, _ := task["requester"].(map[string]any)
	assert.Equal(t, callerFixture, requester["reference"], "the caller must be the requester")
	subject, _ := task["for"].(map[string]any)
	assert.Equal(t, "Patient/pat-1", subject["reference"], "the Task must target the patient")
}
