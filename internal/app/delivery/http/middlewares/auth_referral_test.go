package middlewares

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/fhir_dto"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func referralCommJSON(id, sender, recipient, batchRef, topicCode string) string {
	ext := ""
	if batchRef != "" {
		ext = fmt.Sprintf(",\"extension\":[{\"url\":\"%s\",\"valueReference\":{\"reference\":\"%s\"}}]", ReferralBatchExtensionURL, batchRef)
	}
	return fmt.Sprintf(
		`{"resourceType":"Communication","id":%q,"status":"completed","sender":{"reference":%q},"recipient":[{"reference":%q}],"topic":{"coding":[{"code":%q}]}%s}`,
		id, sender, recipient, topicCode, ext,
	)
}

// B3: referral Communication PUTs must be validated — a deterministic id
// derived from recipient|sender|batch, a research-referral topic, a
// well-formed batch extension, and a recipient matching the session identity.

func TestReferralIDFor_Deterministic(t *testing.T) {
	a := referralIDFor("referee-1", "DG3F3STPYZ6HX25A", "batch-1")
	b := referralIDFor("referee-1", "DG3F3STPYZ6HX25A", "batch-1")
	assert.Equal(t, a, b)
	assert.True(t, len(a) > len("referral-")+32, "id should carry a sha256 hex digest")
	c := referralIDFor("referee-2", "DG3F3STPYZ6HX25A", "batch-1")
	d := referralIDFor("referee-1", "DG3F3STPYZ6HX25A", "batch-2")
	assert.NotEqual(t, a, c)
	assert.NotEqual(t, a, d)
}

func TestValidateReferralCommunicationBody_ValidPasses(t *testing.T) {
	sender := "Patient/DG3F3STPYZ6HX25A"
	recipient := "Patient/referee-1"
	batch := "PlanDefinition/batch-1"
	id := referralIDFor("referee-1", "DG3F3STPYZ6HX25A", "batch-1")
	body := referralCommJSON(id, sender, recipient, batch, "research-referral")

	senderID, recipientID, batchID, ok := validateReferralCommunicationBody([]byte(body), id, "referee-1")
	assert.True(t, ok)
	assert.Equal(t, "DG3F3STPYZ6HX25A", senderID)
	assert.Equal(t, "referee-1", recipientID)
	assert.Equal(t, "batch-1", batchID)
}

func TestValidateReferralCommunicationBody_ForgedIDRejected(t *testing.T) {
	// A referral- prefixed urlID that does not match the body's content hash.
	urlID := "referral-deadbeefdeadbeef"
	body := referralCommJSON(urlID, "Patient/DG3F3STPYZ6HX25A", "Patient/referee-1", "PlanDefinition/batch-1", "research-referral")
	_, _, _, ok := validateReferralCommunicationBody([]byte(body), urlID, "referee-1")
	assert.False(t, ok)
}

func TestValidateReferralCommunicationBody_RecipientMismatchRejected(t *testing.T) {
	id := referralIDFor("referee-1", "DG3F3STPYZ6HX25A", "batch-1")
	body := referralCommJSON(id, "Patient/DG3F3STPYZ6HX25A", "Patient/other", "PlanDefinition/batch-1", "research-referral")
	_, _, _, ok := validateReferralCommunicationBody([]byte(body), id, "referee-1")
	assert.False(t, ok)
}

func TestValidateReferralCommunicationBody_WrongTopicRejected(t *testing.T) {
	id := referralIDFor("referee-1", "DG3F3STPYZ6HX25A", "batch-1")
	body := referralCommJSON(id, "Patient/DG3F3STPYZ6HX25A", "Patient/referee-1", "PlanDefinition/batch-1", "general")
	_, _, _, ok := validateReferralCommunicationBody([]byte(body), id, "referee-1")
	assert.False(t, ok)
}

func TestValidateReferralCommunicationBody_MissingBatchExtensionRejected(t *testing.T) {
	id := referralIDFor("referee-1", "DG3F3STPYZ6HX25A", "batch-1")
	body := referralCommJSON(id, "Patient/DG3F3STPYZ6HX25A", "Patient/referee-1", "", "research-referral")
	_, _, _, ok := validateReferralCommunicationBody([]byte(body), id, "referee-1")
	assert.False(t, ok)
}

func TestValidateReferralCommunicationBody_SenderNotPatientRejected(t *testing.T) {
	id := referralIDFor("referee-1", "x", "batch-1")
	body := referralCommJSON(id, "Practitioner/prac-1", "Patient/referee-1", "PlanDefinition/batch-1", "research-referral")
	_, _, _, ok := validateReferralCommunicationBody([]byte(body), id, "referee-1")
	assert.False(t, ok)
}

func TestValidateReferralCommunicationBody_NonReferralIDRejected(t *testing.T) {
	body := referralCommJSON("some-other-id", "Patient/DG3F3STPYZ6HX25A", "Patient/referee-1", "PlanDefinition/batch-1", "research-referral")
	_, _, _, ok := validateReferralCommunicationBody([]byte(body), "some-other-id", "referee-1")
	assert.False(t, ok, "non-referral- prefixed ids must not be treated as referral Communications")
}

// --- B3 wiring: validateReferralCommunication with live checks (mocked clients) ---

// mockReferralPatientClient wraps mockPatientClient with a configurable
// FindPatientByID so the sender-exists check can be exercised.
type mockReferralPatientClient struct {
	*mockPatientClient
	patientExists bool
	findErr       error
}

func (m *mockReferralPatientClient) FindPatientByID(_ context.Context, _ string) (*fhir_dto.Patient, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	if !m.patientExists {
		return nil, errors.New("patient not found")
	}
	return &fhir_dto.Patient{ID: "DG3F3STPYZ6HX25A"}, nil
}

// mockPlanDefinitionClient implements contracts.PlanDefinitionFinder.
type mockPlanDefinitionClient struct {
	batchExists bool
	err         error
}

func (m *mockPlanDefinitionClient) FindPlanDefinitionByID(_ context.Context, _ string) (*fhir_dto.PlanDefinition, error) {
	if m.err != nil {
		return nil, m.err
	}
	if !m.batchExists {
		return nil, errors.New("plan definition not found")
	}
	return &fhir_dto.PlanDefinition{ID: "batch-1"}, nil
}

func newReferralTestMW() *Middlewares {
	return &Middlewares{
		PatientFhirClient: &mockReferralPatientClient{
			mockPatientClient: &mockPatientClient{},
			patientExists:     true,
		},
		PlanDefinitionFinder: &mockPlanDefinitionClient{batchExists: true},
	}
}

func validReferralBody() (id, body string) {
	sender := "Patient/DG3F3STPYZ6HX25A"
	batch := "PlanDefinition/batch-1"
	id = referralIDFor("referee-1", "DG3F3STPYZ6HX25A", "batch-1")
	body = referralCommJSON(id, sender, "Patient/referee-1", batch, "research-referral")
	return id, body
}

func TestValidateReferralCommunication_ValidPatientToPatient(t *testing.T) {
	mw := newReferralTestMW()
	id, body := validReferralBody()
	err := mw.validateReferralCommunication(context.Background(), []string{constvars.KonsulinRolePatient}, "referee-1", id, []byte(body))
	assert.NoError(t, err)
}

// rejectionError asserts err is a *referralForbiddenError (mapped to 403 by the
// Auth middleware) and returns it for further checks.
func rejectionError(t *testing.T, err error) *referralForbiddenError {
	t.Helper()
	require.Error(t, err)
	var rfErr *referralForbiddenError
	require.ErrorAs(t, err, &rfErr, "referral rejections must map to 403, not the default 401")
	return rfErr
}

func TestValidateReferralCommunication_RejectionsAreForbidden(t *testing.T) {
	mw := newReferralTestMW()
	t.Run("guest", func(t *testing.T) {
		id, body := validReferralBody()
		rejectionError(t, mw.validateReferralCommunication(context.Background(), []string{constvars.KonsulinRoleGuest}, "", id, []byte(body)))
	})
	t.Run("practitioner", func(t *testing.T) {
		id, body := validReferralBody()
		rejectionError(t, mw.validateReferralCommunication(context.Background(), []string{constvars.KonsulinRolePractitioner}, "prac-1", id, []byte(body)))
	})
	t.Run("forged hash", func(t *testing.T) {
		urlID := "referral-" + "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef"
		body := referralCommJSON(urlID, "Patient/DG3F3STPYZ6HX25A", "Patient/referee-1", "PlanDefinition/batch-1", "research-referral")
		rejectionError(t, mw.validateReferralCommunication(context.Background(), []string{constvars.KonsulinRolePatient}, "referee-1", urlID, []byte(body)))
	})
	t.Run("unknown batch", func(t *testing.T) {
		mw2 := newReferralTestMW()
		mw2.PlanDefinitionFinder = &mockPlanDefinitionClient{batchExists: false}
		id, body := validReferralBody()
		rejectionError(t, mw2.validateReferralCommunication(context.Background(), []string{constvars.KonsulinRolePatient}, "referee-1", id, []byte(body)))
	})
	t.Run("non-referral id", func(t *testing.T) {
		body := referralCommJSON("some-other-id", "Patient/DG3F3STPYZ6HX25A", "Patient/referee-1", "PlanDefinition/batch-1", "research-referral")
		rejectionError(t, mw.validateReferralCommunication(context.Background(), []string{constvars.KonsulinRolePatient}, "referee-1", "some-other-id", []byte(body)))
	})
}

func TestRejectReferralPOST_RejectionIsForbidden(t *testing.T) {
	id, _ := validReferralBody()
	rejectionError(t, rejectReferralPOST(constvars.MethodPost, id))
}

// --- B3 wiring: handleAuthSingleResource end-to-end through the gate ---

// newWiringMW returns a Middlewares whose referral paths never reach the
// enforcer (validated referral PUTs are short-circuited in checkSingle before
// any enforcer call), so a nil Enforcer is safe here.
func newWiringMW() *Middlewares {
	mw := newReferralTestMW()
	mw.Enforcer = nil
	return mw
}

func newAuthSingleResourceRequest(t *testing.T, method, path, body string) (*http.Request, context.Context) {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	ctx := context.WithValue(req.Context(), keyFHIRID, "referee-1")
	ctx = context.WithValue(ctx, keyRoles, []string{constvars.KonsulinRolePatient})
	return req, ctx
}

func TestHandleAuthSingleResource_ValidReferralPUTPasses(t *testing.T) {
	mw := newWiringMW()
	id, body := validReferralBody()
	req, ctx := newAuthSingleResourceRequest(t, http.MethodPut, "/fhir/Communication/"+id, body)

	err := mw.handleAuthSingleResource(ctx, req, []string{constvars.KonsulinRolePatient})
	assert.NoError(t, err, "a valid patient referral PUT must pass the gate and RBAC dispatch")
}

func TestHandleAuthSingleResource_InvalidReferralPUTRejected(t *testing.T) {
	mw := newWiringMW()
	id, _ := validReferralBody()
	body := referralCommJSON(id, "Patient/DG3F3STPYZ6HX25A", "Patient/referee-1", "PlanDefinition/batch-1", "general") // wrong topic
	req, ctx := newAuthSingleResourceRequest(t, http.MethodPut, "/fhir/Communication/"+id, body)

	err := mw.handleAuthSingleResource(ctx, req, []string{constvars.KonsulinRolePatient})
	rejectionError(t, err)
	assert.Contains(t, err.Error(), "invalid referral")
}

func TestHandleAuthSingleResource_ReferralPOSTRejected(t *testing.T) {
	mw := newWiringMW()
	_, body := validReferralBody()
	req, ctx := newAuthSingleResourceRequest(t, http.MethodPost, "/fhir/Communication", body)

	err := mw.handleAuthSingleResource(ctx, req, []string{constvars.KonsulinRolePatient})
	rejectionError(t, err)
	assert.Contains(t, err.Error(), "POST")
}

func TestHandleAuthSingleResource_NonReferralCommunicationPUTStillForbidden(t *testing.T) {
	// A non-referral Communication PUT by a patient is not referral-validated and
	// must remain rejected by RBAC (no policy grants Communication PUTs).
	mw := newWiringMW()
	req, ctx := newAuthSingleResourceRequest(t, http.MethodPut, "/fhir/Communication/some-other-id",
		`{"resourceType":"Communication","id":"some-other-id","status":"completed"}`)

	err := mw.handleAuthSingleResource(ctx, req, []string{constvars.KonsulinRolePatient})
	assert.Error(t, err, "non-referral Communication writes must stay forbidden")
}

func TestValidateReferralCommunication_GuestRejected(t *testing.T) {
	mw := newReferralTestMW()
	id, body := validReferralBody()
	err := mw.validateReferralCommunication(context.Background(), []string{constvars.KonsulinRoleGuest}, "", id, []byte(body))
	assert.Error(t, err)
}

func TestValidateReferralCommunication_PractitionerRejected(t *testing.T) {
	mw := newReferralTestMW()
	id, body := validReferralBody()
	err := mw.validateReferralCommunication(context.Background(), []string{constvars.KonsulinRolePractitioner}, "prac-1", id, []byte(body))
	assert.Error(t, err)
}

func TestValidateReferralCommunication_ForgedHashRejected(t *testing.T) {
	mw := newReferralTestMW()
	// A referral- prefixed urlID whose hash does not match the body content.
	urlID := "referral-" + "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef"
	body := referralCommJSON(urlID, "Patient/DG3F3STPYZ6HX25A", "Patient/referee-1", "PlanDefinition/batch-1", "research-referral")
	err := mw.validateReferralCommunication(context.Background(), []string{constvars.KonsulinRolePatient}, "referee-1", urlID, []byte(body))
	assert.Error(t, err)
}

func TestValidateReferralCommunication_WrongTopicRejected(t *testing.T) {
	mw := newReferralTestMW()
	id, _ := validReferralBody()
	body := referralCommJSON(id, "Patient/DG3F3STPYZ6HX25A", "Patient/referee-1", "PlanDefinition/batch-1", "general")
	err := mw.validateReferralCommunication(context.Background(), []string{constvars.KonsulinRolePatient}, "referee-1", id, []byte(body))
	assert.Error(t, err)
}

func TestValidateReferralCommunication_MissingBatchExtensionRejected(t *testing.T) {
	mw := newReferralTestMW()
	id, _ := validReferralBody()
	body := referralCommJSON(id, "Patient/DG3F3STPYZ6HX25A", "Patient/referee-1", "", "research-referral")
	err := mw.validateReferralCommunication(context.Background(), []string{constvars.KonsulinRolePatient}, "referee-1", id, []byte(body))
	assert.Error(t, err)
}

func TestValidateReferralCommunication_RecipientMismatchRejected(t *testing.T) {
	mw := newReferralTestMW()
	id, _ := validReferralBody()
	body := referralCommJSON(id, "Patient/DG3F3STPYZ6HX25A", "Patient/other", "PlanDefinition/batch-1", "research-referral")
	err := mw.validateReferralCommunication(context.Background(), []string{constvars.KonsulinRolePatient}, "referee-1", id, []byte(body))
	assert.Error(t, err)
}

func TestValidateReferralCommunication_SenderNotPatientRejected(t *testing.T) {
	mw := newReferralTestMW()
	id, _ := validReferralBody()
	body := referralCommJSON(id, "Practitioner/prac-1", "Patient/referee-1", "PlanDefinition/batch-1", "research-referral")
	err := mw.validateReferralCommunication(context.Background(), []string{constvars.KonsulinRolePatient}, "referee-1", id, []byte(body))
	assert.Error(t, err)
}

func TestValidateReferralCommunication_SenderNotRegisteredRejected(t *testing.T) {
	mw := newReferralTestMW()
	mw.PatientFhirClient = &mockReferralPatientClient{
		mockPatientClient: &mockPatientClient{},
		patientExists:     false,
	}
	id, body := validReferralBody()
	err := mw.validateReferralCommunication(context.Background(), []string{constvars.KonsulinRolePatient}, "referee-1", id, []byte(body))
	assert.Error(t, err)
}

func TestValidateReferralCommunication_BatchNotFoundRejected(t *testing.T) {
	mw := newReferralTestMW()
	mw.PlanDefinitionFinder = &mockPlanDefinitionClient{batchExists: false}
	id, body := validReferralBody()
	err := mw.validateReferralCommunication(context.Background(), []string{constvars.KonsulinRolePatient}, "referee-1", id, []byte(body))
	assert.Error(t, err)
}

func TestValidateReferralCommunication_NonReferralIDRejected(t *testing.T) {
	mw := newReferralTestMW()
	body := referralCommJSON("some-other-id", "Patient/DG3F3STPYZ6HX25A", "Patient/referee-1", "PlanDefinition/batch-1", "research-referral")
	err := mw.validateReferralCommunication(context.Background(), []string{constvars.KonsulinRolePatient}, "referee-1", "some-other-id", []byte(body))
	assert.Error(t, err, "non-referral- ids must not pass the referral validator")
}

// --- B3: POST creates carrying a referral- id must be rejected (no forging via create) ---

func TestRejectReferralPOST_RejectsReferralID(t *testing.T) {
	id, _ := validReferralBody()
	assert.Error(t, rejectReferralPOST(constvars.MethodPost, id))
}

func TestRejectReferralPOST_AllowsNonReferralID(t *testing.T) {
	assert.NoError(t, rejectReferralPOST(constvars.MethodPost, "qr-123"))
}

func TestRejectReferralPOST_AllowsPUT(t *testing.T) {
	id, _ := validReferralBody()
	assert.NoError(t, rejectReferralPOST(constvars.MethodPut, id))
}

func TestIsReferralID(t *testing.T) {
	assert.True(t, isReferralID("referral-abc123"))
	assert.False(t, isReferralID("Communication"))
	assert.False(t, isReferralID(""))
}

var _ contracts.PlanDefinitionFinder = (*mockPlanDefinitionClient)(nil)
