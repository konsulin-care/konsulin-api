package controllers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/app/delivery/http/middlewares"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/fhir_dto"

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

func newPurgeTestController(usecase contracts.PatientDataPurger) *PurgeController {
	mw := &middlewares.Middlewares{
		PatientFhirClient: &ctrlMockPatientClient{
			patients: []fhir_dto.Patient{{ID: "pat-1"}},
		},
		PractitionerFhirClient: &ctrlMockPractitionerClient{},
	}
	return NewPurgeController(usecase, mw, zap.NewNop())
}

// newPurgeTestControllerWithPractitioner resolves the session to a Practitioner
// FHIR identity (practitioner-first resolution wins).
func newPurgeTestControllerWithPractitioner(usecase contracts.PatientDataPurger) *PurgeController {
	mw := &middlewares.Middlewares{
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
	c := newPurgeTestController(usecase)

	rec := doPurgeRequest(t, c, []string{constvars.KonsulinRolePatient}, "user-123")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, []string{"pat-1"}, usecase.fhirIDs, "only the session patient's own fhirID may be purged")
	assert.Equal(t, []string{"user-123"}, usecase.uids)
}

func TestPurgeData_GuestSessionForbidden(t *testing.T) {
	usecase := &ctrlMockPatientDataPurger{}
	c := newPurgeTestController(usecase)

	rec := doPurgeRequest(t, c, []string{constvars.KonsulinRoleGuest}, "anonymous")
	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Empty(t, usecase.fhirIDs)
}

func TestPurgeData_PractitionerSessionForbidden(t *testing.T) {
	usecase := &ctrlMockPatientDataPurger{}
	c := newPurgeTestControllerWithPractitioner(usecase)

	rec := doPurgeRequest(t, c, []string{constvars.KonsulinRolePractitioner}, "user-456")
	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Empty(t, usecase.fhirIDs)
}

func TestPurgeData_NoSessionForbidden(t *testing.T) {
	usecase := &ctrlMockPatientDataPurger{}
	c := newPurgeTestController(usecase)

	rec := doPurgeRequest(t, c, nil, "")
	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Empty(t, usecase.fhirIDs)
}

func TestPurgeData_UsecaseErrorReturns500(t *testing.T) {
	usecase := &ctrlMockPatientDataPurger{err: errors.New("fhir down")}
	c := newPurgeTestController(usecase)

	rec := doPurgeRequest(t, c, []string{constvars.KonsulinRolePatient}, "user-123")
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Equal(t, []string{"pat-1"}, usecase.fhirIDs)
}

var _ contracts.PatientFhirClient = (*ctrlMockPatientClient)(nil)
var _ contracts.PractitionerFhirClient = (*ctrlMockPractitionerClient)(nil)
