package referrals

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/fhir_http_client"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// confirm fixtures: the Task id, the confirmed candidate, the gateway
// global key and the created local ServiceRequest id.
const (
	confirmTaskID     = "task-in-1"
	confirmPatientID  = "pat-email"
	confirmGlobalKey  = "01HZYGLOBALKEY000001"
	confirmLocalSRID  = "sr-local-9"
	confirmPurpose    = "cardiology-handoff"
	confirmPatientRaw = `{"resourceType":"Patient","id":"pat-email",` +
		`"telecom":[{"system":"email","value":"ada@example.com"}]}`
	confirmTaskRaw = `{"resourceType":"Task","id":"task-in-1",` +
		`"status":"requested","intent":"order",` +
		`"description":"cardiology-handoff",` +
		`"focus":{"reference":"ServiceRequest/sr-1"},` +
		`"input":[` +
		`{"type":{"text":"candidate-patient"},"valueReference":{"reference":"Patient/pat-email"}},` +
		`{"type":{"text":"candidate-patient"},"valueReference":{"reference":"Patient/pat-name"}}]}`
)

// fakeLinker is an in-memory identity plane: Resolve serves a fixed
// candidate list, Link records every call and answers success.
type fakeLinker struct {
	// resolveCands is served for every Resolve call.
	resolveCands []contracts.IdentityCandidate
	// resolveCalls counts Resolve invocations.
	resolveCalls int
	// links records every Link invocation in order.
	links []linkCall
}

// linkCall is one recorded Link invocation.
type linkCall struct {
	// GlobalKey is the confirmed candidate key presented.
	GlobalKey string
	// LocalResourceID is the party-local resource linked.
	LocalResourceID string
}

// Resolve serves the fixed candidate list and counts the call.
func (f *fakeLinker) Resolve(context.Context, string, string) ([]contracts.IdentityCandidate, error) {
	f.resolveCalls++
	return f.resolveCands, nil
}

// Link records the invocation and reports the equivalence row written.
func (f *fakeLinker) Link(_ context.Context, globalKey, localResourceID string) (bool, error) {
	f.links = append(f.links, linkCall{
		GlobalKey:       globalKey,
		LocalResourceID: localResourceID,
	})
	return true, nil
}

// confirmFixture wires a Confirmer over the fake identity plane and an
// httptest FHIR server serving the intake Task and confirmed Patient and
// recording every write.
type confirmFixture struct {
	// confirmer is the unit under test.
	confirmer *Confirmer
	// identities is the fake identity plane.
	identities *fakeLinker
	// srBody is the raw body received at POST /ServiceRequest.
	srBody []byte
	// taskBody is the raw body received at PUT /Task/task-in-1.
	taskBody []byte
}

// newConfirmFixture builds the confirmer over the fake identity plane
// (one foreign candidate carrying the global key), a fresh federation
// pepper delivered through APP_PEPPER (fresh APP_DATA_DIR so no sealed
// pepper file interferes) and an httptest FHIR server serving the intake
// Task, the confirmed Patient and the create/write responses.
func newConfirmFixture(t *testing.T) *confirmFixture {
	t.Helper()
	fx := &confirmFixture{
		identities: &fakeLinker{resolveCands: []contracts.IdentityCandidate{{
			GlobalKey:       confirmGlobalKey,
			PartyID:         "party-sender",
			LocalResourceID: "Patient/ext-9",
			MatchField:      "email",
		}}},
	}

	pepper := make([]byte, 32)
	for i := range pepper {
		pepper[i] = byte(i + 1)
	}
	t.Setenv("APP_DATA_DIR", t.TempDir())
	t.Setenv("APP_PEPPER", base64.StdEncoding.EncodeToString(pepper))

	fhir := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/fhir+json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/"+resourceTask+"/"+confirmTaskID:
			_, _ = w.Write([]byte(confirmTaskRaw))
		case r.Method == http.MethodGet && r.URL.Path == "/"+constvars.ResourcePatient+"/"+confirmPatientID:
			_, _ = w.Write([]byte(confirmPatientRaw))
		case r.Method == http.MethodPost && r.URL.Path == "/"+constvars.ResourceServiceRequest:
			fx.srBody, _ = io.ReadAll(r.Body)
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"resourceType":"ServiceRequest","id":"` +
				confirmLocalSRID + `"}`))
		case r.Method == http.MethodPut && r.URL.Path == "/"+resourceTask+"/"+confirmTaskID:
			fx.taskBody, _ = io.ReadAll(r.Body)
			_, _ = w.Write(fx.taskBody)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(fhir.Close)

	fx.confirmer = NewConfirmer(ConfirmerDeps{
		Identities:  fx.identities,
		FHIRClient:  fhir_http_client.New(zap.NewNop()),
		FHIRBaseURL: fhir.URL,
		Logger:      zap.NewNop(),
	})
	return fx
}

// parseBody decodes one recorded write body into a generic document.
func parseBody(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	var doc map[string]any
	require.NoError(t, json.Unmarshal(raw, &doc), "the recorded body must be JSON")
	return doc
}

// referenceOf reads one reference member out of a generic document.
func referenceOf(t *testing.T, doc map[string]any, key string) string {
	t.Helper()
	ref, _ := doc[key].(map[string]any)
	require.NotNil(t, ref, key+" must be a reference object")
	value, _ := ref["reference"].(string)
	return value
}

// TestConfirm_LinksIdentityCreatesServiceRequestAndAcceptsTask covers
// expected output 1: confirming a candidate links the identity via the
// gateway, creates the local ServiceRequest and accepts the Task.
// Parameters: t is the test harness.
// Returns nothing; fails on any deviation.
func TestConfirm_LinksIdentityCreatesServiceRequestAndAcceptsTask(t *testing.T) {
	fx := newConfirmFixture(t)

	srID, err := fx.confirmer.Confirm(context.Background(), confirmTaskID, confirmPatientID)

	require.NoError(t, err, "confirming a listed candidate must succeed")
	assert.Equal(t, confirmLocalSRID, srID, "the created ServiceRequest id must be returned")

	// The identity is linked exactly once, at the gateway, for the
	// confirmed candidate's global key and the local patient.
	require.Len(t, fx.identities.links, 1, "exactly one link call must be issued")
	assert.Equal(t, confirmGlobalKey, fx.identities.links[0].GlobalKey,
		"the confirmed candidate's global key must be linked")
	assert.Equal(t, constvars.FHIRRefPrefixPatient+confirmPatientID,
		fx.identities.links[0].LocalResourceID, "the local patient must be the link target")
	assert.Equal(t, 1, fx.identities.resolveCalls, "the global key must come from one resolve")

	// The local ServiceRequest is created for the confirmed patient.
	require.NotEmpty(t, fx.srBody, "a local ServiceRequest must be created")
	sr := parseBody(t, fx.srBody)
	assert.Equal(t, constvars.ResourceServiceRequest, sr["resourceType"],
		"a ServiceRequest must be created")
	assert.Equal(t, "active", sr["status"], "the local order must be active")
	assert.Equal(t, taskIntent, sr["intent"], "the local order must be an order")
	assert.Equal(t, constvars.FHIRRefPrefixPatient+confirmPatientID,
		referenceOf(t, sr, "subject"), "the order must belong to the confirmed patient")

	// The Task is accepted, assigned to the confirmed patient and focused
	// on the created local ServiceRequest.
	require.NotEmpty(t, fx.taskBody, "the Task must be written back")
	task := parseBody(t, fx.taskBody)
	assert.Equal(t, taskStatusAccepted, task["status"], "the Task must be accepted")
	assert.Equal(t, constvars.FHIRRefPrefixPatient+confirmPatientID,
		referenceOf(t, task, "for"), "the Task must be assigned to the confirmed patient")
	assert.Equal(t, constvars.ResourceServiceRequest+"/"+confirmLocalSRID,
		referenceOf(t, task, "focus"), "the Task must focus the local ServiceRequest")
	assert.Equal(t, confirmPurpose, task["description"],
		"the referral purpose must survive the update")
}

// TestReject_MarksTaskRejectedWithoutLink covers expected output 2:
// rejecting marks the Task rejected without any link call.
// Parameters: t is the test harness.
// Returns nothing; fails on any deviation.
func TestReject_MarksTaskRejectedWithoutLink(t *testing.T) {
	fx := newConfirmFixture(t)

	err := fx.confirmer.Reject(context.Background(), confirmTaskID)

	require.NoError(t, err, "rejecting an intake Task must succeed")
	assert.Empty(t, fx.identities.links,
		"rejecting must never call the gateway link operation")
	assert.Zero(t, fx.identities.resolveCalls,
		"rejecting must never touch the identity plane")
	assert.Empty(t, fx.srBody, "rejecting must never create a ServiceRequest")

	require.NotEmpty(t, fx.taskBody, "the Task must be written back")
	task := parseBody(t, fx.taskBody)
	assert.Equal(t, taskStatusRejected, task["status"], "the Task must be rejected")
	assert.Equal(t, confirmPurpose, task["description"],
		"the referral purpose must survive the update")
}
