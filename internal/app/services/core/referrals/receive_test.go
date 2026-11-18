package referrals

import (
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/konsulin-care/konsulin-api/internal/pkg/constvars"
	"github.com/konsulin-care/konsulin-api/internal/pkg/fhir_dto"
	"github.com/konsulin-care/konsulin-api/internal/pkg/fhir_http_client"

	"github.com/konsulin-care/gateway-sdk-go/envelope"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// receive fixtures: the parties, the ticket purpose and the incoming order.
const (
	receiveSelf     = "party-self"
	receiveSender   = "party-sender"
	receivePurpose  = "cardiology-handoff"
	receiveKid      = "kid-self"
	receiveEmail    = "Ada@Example.COM"
	receiveLocalPtx = "ada@example.com"
)

// receiveErrVerifier serves a fixed rejection (invalid or replayed ticket)
// and records what ReceiveReferral presented.
type receiveErrVerifier struct {
	// err is served for every VerifyTicket call.
	err error
	// mu guards the recorded presentation.
	mu sync.Mutex
	// gotToken is the ticket of the last call.
	gotToken string
	// gotScope is the expected scope of the last call.
	gotScope string
}

// VerifyTicket records the presentation and serves the fixed rejection.
func (v *receiveErrVerifier) VerifyTicket(_ context.Context, token, expectedScope string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.gotToken = token
	v.gotScope = expectedScope
	return v.err
}

// receiveFixture wires a Receiver over an accept-all ticket verifier, a
// real X25519 use:enc key pair and an httptest FHIR server that serves the
// soft-match Patient searches and records every write.
type receiveFixture struct {
	// receiver is the unit under test.
	receiver *Receiver
	// verifier is the fake ticket verifier.
	verifier *receiveErrVerifier
	// writes records every non-GET FHIR request path.
	writes []string
	// taskBody is the raw body the FHIR server received at POST /Task.
	taskBody []byte
	// searches records the Patient search queries received.
	searches []string
	// key is the receiver's own use:enc key (the seal target in tests).
	key *ecdh.PrivateKey
}

// newReceiveFixture builds the receiver: an accept-all verifier, a fresh
// X25519 key, the party ids, a pepper delivered through APP_PEPPER (fresh
// APP_DATA_DIR so no sealed pepper file interferes) and an httptest FHIR
// server. The email search returns one local patient carrying the incoming
// email under a different name; the name/birthDate search returns another
// local patient matching name and birthDate; the phone search returns
// nothing.
func newReceiveFixture(t *testing.T) *receiveFixture {
	t.Helper()
	fx := &receiveFixture{verifier: &receiveErrVerifier{err: nil}}

	pepper := make([]byte, 32)
	for i := range pepper {
		pepper[i] = byte(i + 1)
	}
	t.Setenv("APP_DATA_DIR", t.TempDir())
	t.Setenv("APP_PEPPER", base64.StdEncoding.EncodeToString(pepper))

	byEmail := fhir_dto.Patient{ID: "pat-email", ResourceType: constvars.ResourcePatient}
	byEmail.Telecom = []fhir_dto.ContactPoint{
		{System: "email", Value: receiveLocalPtx},
	}
	byEmail.Name = []fhir_dto.HumanName{{Family: "Smith", Given: []string{"Alice"}}}
	byEmail.BirthDate = "1985-01-01"

	byName := fhir_dto.Patient{ID: "pat-name", ResourceType: constvars.ResourcePatient}
	byName.Name = []fhir_dto.HumanName{{Use: "official", Family: "Lovelace", Given: []string{"Ada"}}}
	byName.BirthDate = "1990-12-10"

	fhir := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method != http.MethodGet:
			fx.writes = append(fx.writes, r.Method+" "+r.URL.Path)
			if r.Method == http.MethodPost && r.URL.Path == "/"+resourceTask {
				fx.taskBody, _ = io.ReadAll(r.Body)
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write([]byte(`{"resourceType":"Task","id":"task-in-1"}`))
				return
			}
			w.WriteHeader(http.StatusCreated)
		case r.URL.Path == "/"+constvars.ResourcePatient:
			q := r.URL.Query()
			fx.searches = append(fx.searches, r.URL.RawQuery)
			switch {
			case q.Get(paramEmail) != "":
				writeBundle(t, w, partnerBundle(mustMarshalEntry(t, byEmail)))
			case q.Get(paramPhone) != "":
				writeBundle(t, w, partnerBundle())
			case q.Get(paramName) != "":
				writeBundle(t, w, partnerBundle(mustMarshalEntry(t, byName)))
			default:
				writeBundle(t, w, partnerBundle())
			}
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(fhir.Close)

	key, err := ecdh.X25519().GenerateKey(rand.Reader)
	require.NoError(t, err, "the fixture must generate a use:enc key")
	fx.receiver = NewReceiver(ReceiverDeps{
		Tickets:       fx.verifier,
		EncPrivateKey: key,
		PartyID:       receiveSelf,
		FHIRClient:    fhir_http_client.New(zap.NewNop()),
		FHIRBaseURL:   fhir.URL,
		Logger:        zap.NewNop(),
	})
	fx.key = key
	return fx
}

// mintReceiveTicket renders the compact ks-approval+jwt presentation: a
// header, the claim set (sub = the sending party, purpose) and a dummy
// signature segment — the fake verifier never checks the signature.
func mintReceiveTicket(t *testing.T, claims map[string]any) string {
	t.Helper()
	raw, err := json.Marshal(claims)
	require.NoError(t, err)
	header := base64.RawURLEncoding.EncodeToString(
		[]byte(`{"alg":"Ed25519","kid":"gw-1","typ":"ks-approval+jwt"}`))
	return header + "." + base64.RawURLEncoding.EncodeToString(raw) + ".c2ln"
}

// sealReceiveEnvelope seals the incoming {patient, serviceRequest} payload
// exactly as the sending partner does: bound from the sender, to this
// party, service "referrals".
func sealReceiveEnvelope(t *testing.T, key *ecdh.PrivateKey, payload referralPayload) string {
	t.Helper()
	data, err := json.Marshal(payload)
	require.NoError(t, err)
	compact, err := envelope.SealBound(data, receiveKid, key.PublicKey(), envelope.Binding{
		From: receiveSender, To: receiveSelf, Service: referralService,
	})
	require.NoError(t, err, "the fixture must seal a bound envelope")
	return compact
}

// receivePayload returns the incoming referral payload: the sealed
// demographics plus the referred order.
func receivePayload() referralPayload {
	patient := referralPatient{
		ID:   "ext-9",
		Name: []fhir_dto.HumanName{{Use: "official", Family: "Lovelace", Given: []string{"Ada"}}},
		Telecom: []fhir_dto.ContactPoint{
			{System: "email", Value: receiveEmail},
			{System: "phone", Value: "6281234567890"},
		},
		BirthDate: "1990-12-10",
	}
	return referralPayload{
		Patient:        patient,
		ServiceRequest: json.RawMessage(serviceReqFacts),
	}
}

// TestReceiveReferral_StoresIncomingTaskWithSoftMatchedCandidates covers
// expected output 1: a valid referral is stored as an incoming Task
// (status "requested") listing the soft-matched candidate patients, and no
// patient is linked.
func TestReceiveReferral_StoresIncomingTaskWithSoftMatchedCandidates(t *testing.T) {
	fx := newReceiveFixture(t)
	ticket := mintReceiveTicket(t, map[string]any{
		"sub": receiveSender, "purpose": receivePurpose,
	})
	sealed := sealReceiveEnvelope(t, fx.key, receivePayload())

	taskID, err := fx.receiver.ReceiveReferral(context.Background(), ticket, sealed)

	require.NoError(t, err, "a valid referral must be accepted")
	assert.Equal(t, "task-in-1", taskID, "the stored Task id must be returned")
	assert.Equal(t, referralScope, fx.verifier.gotScope,
		"the ticket must be verified for the referral scope")
	assert.Equal(t, ticket, fx.verifier.gotToken, "the presented ticket must be verified")

	// The incoming Task is stored as requested, focusing the referred order.
	var task map[string]any
	require.NoError(t, json.Unmarshal(fx.taskBody, &task), "the incoming Task must be recorded")
	assert.Equal(t, resourceTask, task["resourceType"], "the record must be a Task")
	assert.Equal(t, taskStatusRequested, task["status"], "the Task must be requested")
	assert.Equal(t, taskIntent, task["intent"], "the Task must stay an order")
	assert.Equal(t, receivePurpose, task["description"], "the purpose must be recorded")
	focus, _ := task["focus"].(map[string]any)
	require.NotNil(t, focus, "the ServiceRequest must be focused")
	assert.Equal(t, "ServiceRequest/sr-1", focus["reference"], "the focus must reference the order")

	// Both strategies contribute their candidate; the Task lists them.
	input, ok := task["input"].([]any)
	require.True(t, ok, "the Task must list candidate inputs")
	candidates := make([]string, 0, len(input))
	for _, raw := range input {
		entry, _ := raw.(map[string]any)
		typeText, _ := entry["type"].(map[string]any)
		assert.Equal(t, candidateType, typeText["text"], "each input must be labeled a candidate")
		ref, _ := entry["valueReference"].(map[string]any)
		candidates = append(candidates, ref["reference"].(string))
	}
	assert.ElementsMatch(t, []string{"Patient/pat-email", "Patient/pat-name"}, candidates,
		"the Task must list the hash-matched and name/birth-matched candidates")

	// Never links automatically: the only FHIR write is the Task create —
	// no Patient create/update and no Task.for subject assignment.
	assert.Equal(t, []string{"POST /" + resourceTask}, fx.writes,
		"only the incoming Task may be written; patients must never be linked")
	_, hasFor := task["for"]
	assert.False(t, hasFor, "the Task must not assign a single linked patient")
}

// TestReceiveReferral_InvalidOrReplayedTicketStoresNothing covers expected
// output 2: an invalid or replayed ticket is rejected and nothing is
// stored — the rejection happens before any FHIR traffic.
func TestReceiveReferral_InvalidOrReplayedTicketStoresNothing(t *testing.T) {
	cases := []struct {
		name string
		err  error
	}{
		{name: "invalid ticket", err: errors.New("signature verification failed")},
		{name: "replayed ticket", err: errors.New("ticket jti already consumed")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fx := newReceiveFixture(t)
			fx.verifier.err = tc.err
			ticket := mintReceiveTicket(t, map[string]any{
				"sub": receiveSender, "purpose": receivePurpose,
			})
			sealed := sealReceiveEnvelope(t, fx.key, receivePayload())

			taskID, err := fx.receiver.ReceiveReferral(context.Background(), ticket, sealed)

			require.Error(t, err, "a rejected ticket must fail")
			assert.ErrorIs(t, err, ErrReferralRejected, "the rejection must carry the sentinel")
			assert.ErrorIs(t, err, tc.err, "the verifier denial must surface")
			assert.Empty(t, taskID, "no Task id must be returned")
			assert.Empty(t, fx.writes, "nothing may be stored for a rejected ticket")
			assert.Empty(t, fx.taskBody, "no Task body may reach the FHIR server")
			assert.Equal(t, ticket, fx.verifier.gotToken, "the presented ticket must be checked")
		})
	}
}
