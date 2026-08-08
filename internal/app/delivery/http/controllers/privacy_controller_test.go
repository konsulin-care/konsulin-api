package controllers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"testing"

	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/app/delivery/http/middlewares"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/fhir_dto"
	"konsulin-service/internal/pkg/utils"

	"github.com/casbin/casbin/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// ctrlMockPatientDataPurger records purge invocations.
type ctrlMockPatientDataPurger struct {
	fhirIDs []string
	uids    []string
	err     error
}

func (m *ctrlMockPatientDataPurger) PurgePatientData(_ context.Context, fhirID, supertokensUserID string) error {
	m.fhirIDs = append(m.fhirIDs, fhirID)
	m.uids = append(m.uids, supertokensUserID)
	return m.err
}

// ctrlMockPatientClient satisfies contracts.PatientFhirClient for identity
// resolution in controller tests.
type ctrlMockPatientClient struct {
	patients []fhir_dto.Patient
	err      error
}

func (m *ctrlMockPatientClient) FindPatientByIdentifier(_ context.Context, _ string) ([]fhir_dto.Patient, error) {
	return m.patients, m.err
}
func (*ctrlMockPatientClient) FindPatientByID(_ context.Context, _ string) (*fhir_dto.Patient, error) {
	return nil, nil
}
func (*ctrlMockPatientClient) FindPatientByEmail(_ context.Context, _ string) ([]fhir_dto.Patient, error) {
	return nil, nil
}
func (*ctrlMockPatientClient) FindPatientByPhone(_ context.Context, _ string) ([]fhir_dto.Patient, error) {
	return nil, nil
}
func (*ctrlMockPatientClient) CreatePatient(_ context.Context, _ *fhir_dto.Patient) (*fhir_dto.Patient, error) {
	return nil, nil
}
func (*ctrlMockPatientClient) UpdatePatient(_ context.Context, _ *fhir_dto.Patient) (*fhir_dto.Patient, error) {
	return nil, nil
}
func (*ctrlMockPatientClient) PatchPatient(_ context.Context, _ *fhir_dto.Patient) (*fhir_dto.Patient, error) {
	return nil, nil
}

// ctrlMockPractitionerClient satisfies contracts.PractitionerFhirClient.
type ctrlMockPractitionerClient struct {
	practitioners []fhir_dto.Practitioner
	err           error
}

func (m *ctrlMockPractitionerClient) FindPractitionerByIdentifier(_ context.Context, _, _ string) ([]fhir_dto.Practitioner, error) {
	return m.practitioners, m.err
}
func (*ctrlMockPractitionerClient) FindPractitionerByID(_ context.Context, _ string) (*fhir_dto.Practitioner, error) {
	return nil, nil
}
func (*ctrlMockPractitionerClient) FindPractitionerByEmail(_ context.Context, _ string) ([]fhir_dto.Practitioner, error) {
	return nil, nil
}
func (*ctrlMockPractitionerClient) FindPractitionerByPhone(_ context.Context, _ string) ([]fhir_dto.Practitioner, error) {
	return nil, nil
}
func (*ctrlMockPractitionerClient) CreatePractitioner(_ context.Context, _ *fhir_dto.Practitioner) (*fhir_dto.Practitioner, error) {
	return nil, nil
}
func (*ctrlMockPractitionerClient) UpdatePractitioner(_ context.Context, _ *fhir_dto.Practitioner) (*fhir_dto.Practitioner, error) {
	return nil, nil
}
func (*ctrlMockPractitionerClient) PatchPractitioner(_ context.Context, _ *fhir_dto.Practitioner) (*fhir_dto.Practitioner, error) {
	return nil, nil
}

// controllerTestEnforcer builds a Casbin enforcer from the repo-root resources,
// mirroring middlewares.testEnforcer (unexported from another package). Test
// working dirs differ from the binary's, so paths are resolved relative to this
// source file.
func controllerTestEnforcer(t testing.TB) *casbin.Enforcer {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot resolve test source path")
	}
	root := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "..", "..")
	enf, err := casbin.NewEnforcer(
		filepath.Join(root, "resources", "rbac_model.conf"),
		filepath.Join(root, "resources", "rbac_policy.csv"),
	)
	if err != nil {
		t.Fatalf("failed to load RBAC policy: %v", err)
	}
	enf.AddFunction("pathMatch", func(args ...interface{}) (interface{}, error) {
		if len(args) != 2 {
			return false, nil
		}
		requestPath, ok1 := args[0].(string)
		policyPath, ok2 := args[1].(string)
		if !ok1 || !ok2 {
			return false, nil
		}
		return utils.PathMatch(requestPath, policyPath), nil
	})
	return enf
}

func newPurgeTestController(t testing.TB, usecase contracts.PatientDataPurger) *PurgeController {
	mw := &middlewares.Middlewares{
		Enforcer: controllerTestEnforcer(t),
		PatientFhirClient: &ctrlMockPatientClient{
			patients: []fhir_dto.Patient{{ID: "pat-1"}},
		},
		PractitionerFhirClient: &ctrlMockPractitionerClient{},
	}
	return NewPurgeController(usecase, mw, zap.NewNop())
}

// newPurgeTestControllerWithPractitioner resolves the session to a Practitioner
// FHIR identity (practitioner-first resolution wins).
func newPurgeTestControllerWithPractitioner(t testing.TB, usecase contracts.PatientDataPurger) *PurgeController {
	mw := &middlewares.Middlewares{
		Enforcer: controllerTestEnforcer(t),
		PatientFhirClient: &ctrlMockPatientClient{
			patients: []fhir_dto.Patient{{ID: "pat-1"}},
		},
		PractitionerFhirClient: &ctrlMockPractitionerClient{
			practitioners: []fhir_dto.Practitioner{{ID: "prac-1"}},
		},
	}
	return NewPurgeController(usecase, mw, zap.NewNop())
}

func doPurgeRequest(t *testing.T, c *PurgeController, roles []string, uid string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/privacy/purge", nil)
	ctx := context.WithValue(req.Context(), constvars.CONTEXT_FHIR_ROLE, roles)
	ctx = context.WithValue(ctx, constvars.CONTEXT_UID, uid)
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	c.PurgeData(rec, req)
	return rec
}

func TestPurgeData_PatientSessionPurgesOwnData(t *testing.T) {
	usecase := &ctrlMockPatientDataPurger{}
	c := newPurgeTestController(t, usecase)

	rec := doPurgeRequest(t, c, []string{constvars.KonsulinRolePatient}, "user-123")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, []string{"pat-1"}, usecase.fhirIDs, "only the session patient's own fhirID may be purged")
	assert.Equal(t, []string{"user-123"}, usecase.uids)
}

func TestPurgeData_GuestSessionForbidden(t *testing.T) {
	usecase := &ctrlMockPatientDataPurger{}
	c := newPurgeTestController(t, usecase)

	rec := doPurgeRequest(t, c, []string{constvars.KonsulinRoleGuest}, "anonymous")
	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Empty(t, usecase.fhirIDs)
}

func TestPurgeData_PractitionerSessionForbidden(t *testing.T) {
	usecase := &ctrlMockPatientDataPurger{}
	c := newPurgeTestControllerWithPractitioner(t, usecase)

	rec := doPurgeRequest(t, c, []string{constvars.KonsulinRolePractitioner}, "user-456")
	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Empty(t, usecase.fhirIDs)
}

func TestPurgeData_NoSessionForbidden(t *testing.T) {
	usecase := &ctrlMockPatientDataPurger{}
	c := newPurgeTestController(t, usecase)

	rec := doPurgeRequest(t, c, nil, "")
	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Empty(t, usecase.fhirIDs)
}

func TestPurgeData_UsecaseErrorReturns500(t *testing.T) {
	usecase := &ctrlMockPatientDataPurger{err: errors.New("fhir down")}
	c := newPurgeTestController(t, usecase)

	rec := doPurgeRequest(t, c, []string{constvars.KonsulinRolePatient}, "user-123")
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Equal(t, []string{"pat-1"}, usecase.fhirIDs)
}

var _ contracts.PatientFhirClient = (*ctrlMockPatientClient)(nil)
var _ contracts.PractitionerFhirClient = (*ctrlMockPractitionerClient)(nil)

// TestPurgeData_NilEnforcerFailsClosed proves the authorization consult: without
// an Enforcer the controller must fail closed even for a valid Patient identity.
func TestPurgeData_NilEnforcerFailsClosed(t *testing.T) {
	usecase := &ctrlMockPatientDataPurger{}
	mw := &middlewares.Middlewares{
		PatientFhirClient:      &ctrlMockPatientClient{patients: []fhir_dto.Patient{{ID: "pat-1"}}},
		PractitionerFhirClient: &ctrlMockPractitionerClient{},
	}
	c := NewPurgeController(usecase, mw, zap.NewNop())

	rec := doPurgeRequest(t, c, []string{constvars.KonsulinRolePatient}, "user-123")
	assert.Equal(t, http.StatusForbidden, rec.Code, "authorization must fail closed without an enforcer")
	assert.Empty(t, usecase.fhirIDs)
}

// TestPurgeData_ClinicAdminPolicyDenied proves the policy drives, not the role
// string: a resolved identity is denied.
func TestPurgeData_ClinicAdminPolicyDenied(t *testing.T) {
	usecase := &ctrlMockPatientDataPurger{}
	c := newPurgeTestController(t, usecase)

	rec := doPurgeRequest(t, c, []string{constvars.KonsulinRoleClinicAdmin}, "admin-1")
	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Empty(t, usecase.fhirIDs)
}
