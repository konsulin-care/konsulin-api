package middlewares

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/fhir_dto"

	"github.com/stretchr/testify/assert"
)

// countingPractitionerClient wraps mockPractitionerClient and counts
// FindPractitionerByIdentifier calls to prove lazy, single-shot resolution of
// the secondary identity.
type countingPractitionerClient struct {
	mockPractitionerClient
	byIdentifierCalls int
}

func (c *countingPractitionerClient) FindPractitionerByIdentifier(ctx context.Context, system, value string) ([]fhir_dto.Practitioner, error) {
	c.byIdentifierCalls++
	return c.mockPractitionerClient.FindPractitionerByIdentifier(ctx, system, value)
}

// dualRoleBundleBody builds a FHIR transaction bundle with one Patient PUT and
// one Practitioner PUT entry, mirroring the /profile save request shape.
func dualRoleBundleBody(patientID, practitionerID string) string {
	return `{"resourceType":"Bundle","type":"transaction","entry":[` +
		`{"resource":{"resourceType":"Patient","id":"` + patientID + `"},"request":{"method":"PUT","url":"Patient/` + patientID + `"}},` +
		`{"resource":{"resourceType":"Practitioner","id":"` + practitionerID + `"},"request":{"method":"PUT","url":"Practitioner/` + practitionerID + `"}}]}`
}

// dualRoleMiddleware builds a Middlewares with a real Casbin enforcer and mock
// FHIR clients where the caller owns Patient "pat-1" and Practitioner "prac-1".
func dualRoleMiddleware(t *testing.T) (*Middlewares, *countingPractitionerClient) {
	t.Helper()
	prac := &countingPractitionerClient{mockPractitionerClient: mockPractitionerClient{
		practitioners: []fhir_dto.Practitioner{{ID: "prac-1"}},
	}}
	mw := &Middlewares{
		Enforcer:               testEnforcer(t),
		PatientFhirClient:      &mockPatientClient{patients: []fhir_dto.Patient{{ID: "pat-1"}}},
		PractitionerFhirClient: prac,
	}
	return mw, prac
}

// dualRoleContext seeds the session context values set by the Auth middleware
// for a dual-role session with active role Patient (fhirID = the Patient ID).
func dualRoleContext() context.Context {
	ctx := context.WithValue(context.Background(), keyFHIRID, "pat-1")
	ctx = context.WithValue(ctx, keyFHIRRole, constvars.KonsulinRolePatient)
	return context.WithValue(ctx, keyUID, "user-123")
}

func TestRBACRequest_FHIRIDForRole(t *testing.T) {
	// Nil map falls back to the single active-role ID (today's behavior).
	req := rbacRequest{fhirID: "pat-1"}
	assert.Equal(t, "pat-1", req.fhirIDForRole(constvars.KonsulinRolePatient))
	assert.Equal(t, "pat-1", req.fhirIDForRole(constvars.KonsulinRolePractitioner))

	// A populated per-role map wins for the role it names; others fall back.
	req = rbacRequest{
		fhirID:        "pat-1",
		fhirIDsByRole: map[string]string{constvars.KonsulinRolePractitioner: "prac-1"},
	}
	assert.Equal(t, "pat-1", req.fhirIDForRole(constvars.KonsulinRolePatient))
	assert.Equal(t, "prac-1", req.fhirIDForRole(constvars.KonsulinRolePractitioner))
}

func TestScanBundle_DualRoleOwnsBothResources(t *testing.T) {
	// A transaction bundle with the caller's own Patient PUT and Practitioner
	// PUT authorizes when the per-role map supplies the practitioner identity.
	enf := testEnforcer(t)
	clients := rbacClients{
		patient:      &mockPatientClient{},
		practitioner: &mockPractitionerClient{},
	}
	roles := []string{
		constvars.KonsulinRolePatient,
		constvars.KonsulinRolePractitioner,
		constvars.KonsulinRoleClinicAdmin,
		constvars.KonsulinRoleResearcher,
	}
	fhirIDsByRole := map[string]string{constvars.KonsulinRolePractitioner: "prac-1"}
	body := dualRoleBundleBody("pat-1", "prac-1")

	err := scanBundle(context.Background(), enf, []byte(body), roles, "pat-1", fhirIDsByRole, clients)
	assert.NoError(t, err)
}

func TestScanBundle_DualRoleCrossIdentityDenied(t *testing.T) {
	// The same bundle is rejected when the Practitioner entry targets a
	// practitioner the caller does not own.
	enf := testEnforcer(t)
	clients := rbacClients{
		patient:      &mockPatientClient{},
		practitioner: &mockPractitionerClient{},
	}
	roles := []string{constvars.KonsulinRolePatient, constvars.KonsulinRolePractitioner}
	fhirIDsByRole := map[string]string{constvars.KonsulinRolePractitioner: "prac-1"}
	body := dualRoleBundleBody("pat-1", "other-prac")

	err := scanBundle(context.Background(), enf, []byte(body), roles, "pat-1", fhirIDsByRole, clients)
	assert.Error(t, err)
}

func TestScanBundle_SingleRoleUnchanged(t *testing.T) {
	// A single-role session with no per-role map behaves exactly as before:
	// its own resource passes, the other role's resource is denied.
	enf := testEnforcer(t)
	clients := rbacClients{
		patient:      &mockPatientClient{},
		practitioner: &mockPractitionerClient{},
	}

	body := `{"resourceType":"Bundle","type":"transaction","entry":[{"resource":{"resourceType":"Patient","id":"pat-1"},"request":{"method":"PUT","url":"Patient/pat-1"}}]}`
	err := scanBundle(context.Background(), enf, []byte(body), []string{constvars.KonsulinRolePatient}, "pat-1", nil, clients)
	assert.NoError(t, err)

	body = `{"resourceType":"Bundle","type":"transaction","entry":[{"resource":{"resourceType":"Practitioner","id":"prac-1"},"request":{"method":"PUT","url":"Practitioner/prac-1"}}]}`
	err = scanBundle(context.Background(), enf, []byte(body), []string{constvars.KonsulinRolePatient}, "pat-1", nil, clients)
	assert.Error(t, err)
}

func TestHoldsDualFHIRRoles(t *testing.T) {
	assert.True(t, holdsDualFHIRRoles([]string{constvars.KonsulinRolePatient, constvars.KonsulinRolePractitioner}))
	assert.True(t, holdsDualFHIRRoles([]string{
		constvars.KonsulinRolePatient,
		constvars.KonsulinRolePractitioner,
		constvars.KonsulinRoleClinicAdmin,
		constvars.KonsulinRoleResearcher,
	}))
	assert.False(t, holdsDualFHIRRoles([]string{constvars.KonsulinRolePatient}))
	assert.False(t, holdsDualFHIRRoles([]string{constvars.KonsulinRolePractitioner}))
	assert.False(t, holdsDualFHIRRoles(nil))
	assert.False(t, holdsDualFHIRRoles([]string{constvars.KonsulinRoleClinicAdmin, constvars.KonsulinRoleResearcher}))
}

func TestResolveSecondaryFHIRID(t *testing.T) {
	mw := &Middlewares{
		PatientFhirClient:      &mockPatientClient{patients: []fhir_dto.Patient{{ID: "pat-1"}}},
		PractitionerFhirClient: &mockPractitionerClient{practitioners: []fhir_dto.Practitioner{{ID: "prac-1"}}},
	}
	ctx := context.Background()
	assert.Equal(t, "prac-1", mw.resolveSecondaryFHIRID(ctx, "user-123", constvars.KonsulinRolePatient))
	assert.Equal(t, "pat-1", mw.resolveSecondaryFHIRID(ctx, "user-123", constvars.KonsulinRolePractitioner))
	assert.Equal(t, "", mw.resolveSecondaryFHIRID(ctx, "user-123", constvars.KonsulinRoleClinicAdmin))

	// Missing identity resolves leniently to "".
	empty := &Middlewares{
		PatientFhirClient:      &mockPatientClient{},
		PractitionerFhirClient: &mockPractitionerClient{},
	}
	assert.Equal(t, "", empty.resolveSecondaryFHIRID(ctx, "user-123", constvars.KonsulinRolePatient))
	assert.Equal(t, "", empty.resolveSecondaryFHIRID(ctx, "user-123", constvars.KonsulinRolePractitioner))
}

func TestHandleAuthBundle_DualRoleRetry(t *testing.T) {
	// The reported failing shape — transaction bundle with Patient PUT +
	// Practitioner PUT, active role Patient — authorizes via the lazy retry,
	// and the secondary identity lookup runs exactly once (not per entry).
	mw, prac := dualRoleMiddleware(t)
	ctx := dualRoleContext()

	body := dualRoleBundleBody("pat-1", "prac-1")
	r := httptest.NewRequest(http.MethodPost, "/fhir", strings.NewReader(body))
	roles := []string{
		constvars.KonsulinRolePatient,
		constvars.KonsulinRolePractitioner,
		constvars.KonsulinRoleClinicAdmin,
		constvars.KonsulinRoleResearcher,
	}

	isBundle, err := mw.handleAuthBundle(ctx, r, roles)
	assert.NoError(t, err)
	assert.True(t, isBundle)
	assert.Equal(t, 1, prac.byIdentifierCalls)
}

func TestHandleAuthBundle_DualRoleCrossIdentityDenied(t *testing.T) {
	// Lazy resolution must not widen access: a Practitioner entry for another
	// practitioner still fails.
	mw, _ := dualRoleMiddleware(t)
	ctx := dualRoleContext()

	body := dualRoleBundleBody("pat-1", "other-prac")
	r := httptest.NewRequest(http.MethodPost, "/fhir", strings.NewReader(body))
	roles := []string{constvars.KonsulinRolePatient, constvars.KonsulinRolePractitioner}

	isBundle, err := mw.handleAuthBundle(ctx, r, roles)
	assert.Error(t, err)
	assert.True(t, isBundle)
}

func TestHandleAuthBundle_DualRoleRetry_ActivePractitioner(t *testing.T) {
	// Symmetric case: active role Practitioner + the same bundle also passes,
	// with the Patient identity resolved lazily on the retry.
	prac := &countingPractitionerClient{mockPractitionerClient: mockPractitionerClient{
		practitioners: []fhir_dto.Practitioner{{ID: "prac-1"}},
	}}
	mw := &Middlewares{
		Enforcer:               testEnforcer(t),
		PatientFhirClient:      &mockPatientClient{patients: []fhir_dto.Patient{{ID: "pat-1"}}},
		PractitionerFhirClient: prac,
	}
	ctx := context.WithValue(context.Background(), keyFHIRID, "prac-1")
	ctx = context.WithValue(ctx, keyFHIRRole, constvars.KonsulinRolePractitioner)
	ctx = context.WithValue(ctx, keyUID, "user-123")

	body := dualRoleBundleBody("pat-1", "prac-1")
	r := httptest.NewRequest(http.MethodPost, "/fhir", strings.NewReader(body))
	roles := []string{constvars.KonsulinRolePatient, constvars.KonsulinRolePractitioner}

	isBundle, err := mw.handleAuthBundle(ctx, r, roles)
	assert.NoError(t, err)
	assert.True(t, isBundle)
	// Active Practitioner resolves the Patient via the patient client, so no
	// practitioner lookup is needed.
	assert.Zero(t, prac.byIdentifierCalls)
}

func TestHandleAuthBundle_SingleRoleNoSecondaryLookup(t *testing.T) {
	// A Patient-only session never triggers the secondary lookup.
	mw, prac := dualRoleMiddleware(t)
	ctx := context.WithValue(context.Background(), keyFHIRID, "pat-1")
	ctx = context.WithValue(ctx, keyFHIRRole, constvars.KonsulinRolePatient)
	ctx = context.WithValue(ctx, keyUID, "user-123")

	body := `{"resourceType":"Bundle","type":"transaction","entry":[{"resource":{"resourceType":"Patient","id":"pat-1"},"request":{"method":"PUT","url":"Patient/pat-1"}}]}`
	r := httptest.NewRequest(http.MethodPost, "/fhir", strings.NewReader(body))

	isBundle, err := mw.handleAuthBundle(ctx, r, []string{constvars.KonsulinRolePatient})
	assert.NoError(t, err)
	assert.True(t, isBundle)
	assert.Zero(t, prac.byIdentifierCalls)
}

func TestHandleAuthSingleResource_DualRoleRetry_ActivePractitioner(t *testing.T) {
	// Symmetric case: active role Practitioner PUTting own Patient resource
	// authorizes via the lazy retry (Patient role owns the target).
	mw := &Middlewares{
		Enforcer:               testEnforcer(t),
		PatientFhirClient:      &mockPatientClient{patients: []fhir_dto.Patient{{ID: "pat-1"}}},
		PractitionerFhirClient: &mockPractitionerClient{practitioners: []fhir_dto.Practitioner{{ID: "prac-1"}}},
	}
	ctx := context.WithValue(context.Background(), keyFHIRID, "prac-1")
	ctx = context.WithValue(ctx, keyFHIRRole, constvars.KonsulinRolePractitioner)
	ctx = context.WithValue(ctx, keyUID, "user-123")

	body := `{"resourceType":"Patient","id":"pat-1"}`
	r := httptest.NewRequest(http.MethodPut, "/fhir/Patient/pat-1", strings.NewReader(body))
	roles := []string{constvars.KonsulinRolePatient, constvars.KonsulinRolePractitioner}

	err := mw.handleAuthSingleResource(ctx, r, roles)
	assert.NoError(t, err)
}

func TestHandleAuthSingleResource_DualRoleCrossIdentityDenied(t *testing.T) {
	// Single-resource retry must not widen access: PUTting another
	// practitioner's resource with active role Patient still fails.
	mw, _ := dualRoleMiddleware(t)
	ctx := dualRoleContext()

	body := `{"resourceType":"Practitioner","id":"other-prac"}`
	r := httptest.NewRequest(http.MethodPut, "/fhir/Practitioner/other-prac", strings.NewReader(body))
	roles := []string{constvars.KonsulinRolePatient, constvars.KonsulinRolePractitioner}

	err := mw.handleAuthSingleResource(ctx, r, roles)
	assert.Error(t, err)
}

func TestHandleAuthSingleResource_DualRoleRetry(t *testing.T) {
	// PUT of the caller's own Practitioner resource authorizes when the active
	// role is Patient, via the single-resource retry.
	mw, prac := dualRoleMiddleware(t)
	ctx := dualRoleContext()

	body := `{"resourceType":"Practitioner","id":"prac-1"}`
	r := httptest.NewRequest(http.MethodPut, "/fhir/Practitioner/prac-1", strings.NewReader(body))
	roles := []string{constvars.KonsulinRolePatient, constvars.KonsulinRolePractitioner}

	err := mw.handleAuthSingleResource(ctx, r, roles)
	assert.NoError(t, err)
	assert.Equal(t, 1, prac.byIdentifierCalls)
}
