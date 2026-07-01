package middlewares

import (
	"context"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/fhir_dto"
	"testing"

	"github.com/stretchr/testify/assert"
)

// mockPatientClient returns configurable results for PatientFhirClient.
type mockPatientClient struct {
	patients []fhir_dto.Patient
	err      error
}

func (m *mockPatientClient) FindPatientByIdentifier(_ context.Context, identifier string) ([]fhir_dto.Patient, error) {
	return m.patients, m.err
}

func (m *mockPatientClient) FindPatientByID(_ context.Context, _ string) (*fhir_dto.Patient, error) {
	return nil, nil
}
func (m *mockPatientClient) FindPatientByEmail(_ context.Context, _ string) ([]fhir_dto.Patient, error) {
	return nil, nil
}
func (m *mockPatientClient) FindPatientByPhone(_ context.Context, _ string) ([]fhir_dto.Patient, error) {
	return nil, nil
}
func (m *mockPatientClient) CreatePatient(_ context.Context, _ *fhir_dto.Patient) (*fhir_dto.Patient, error) {
	return nil, nil
}
func (m *mockPatientClient) UpdatePatient(_ context.Context, _ *fhir_dto.Patient) (*fhir_dto.Patient, error) {
	return nil, nil
}
func (m *mockPatientClient) PatchPatient(_ context.Context, _ *fhir_dto.Patient) (*fhir_dto.Patient, error) {
	return nil, nil
}

// mockPractitionerClient returns configurable results for PractitionerFhirClient.
type mockPractitionerClient struct {
	practitioners []fhir_dto.Practitioner
	err           error
}

func (m *mockPractitionerClient) FindPractitionerByIdentifier(_ context.Context, system, value string) ([]fhir_dto.Practitioner, error) {
	return m.practitioners, m.err
}

func (m *mockPractitionerClient) FindPractitionerByID(_ context.Context, _ string) (*fhir_dto.Practitioner, error) {
	return nil, nil
}
func (m *mockPractitionerClient) FindPractitionerByEmail(_ context.Context, _ string) ([]fhir_dto.Practitioner, error) {
	return nil, nil
}
func (m *mockPractitionerClient) FindPractitionerByPhone(_ context.Context, _ string) ([]fhir_dto.Practitioner, error) {
	return nil, nil
}
func (m *mockPractitionerClient) CreatePractitioner(_ context.Context, _ *fhir_dto.Practitioner) (*fhir_dto.Practitioner, error) {
	return nil, nil
}
func (m *mockPractitionerClient) UpdatePractitioner(_ context.Context, _ *fhir_dto.Practitioner) (*fhir_dto.Practitioner, error) {
	return nil, nil
}
func (m *mockPractitionerClient) PatchPractitioner(_ context.Context, _ *fhir_dto.Practitioner) (*fhir_dto.Practitioner, error) {
	return nil, nil
}

func TestResolveFHIRIdentity_ActiveRolePatient(t *testing.T) {
	// When activeRole is "Patient" and a Patient resource exists, resolveFHIRIdentity
	// should return the Patient role even if a Practitioner resource also exists.
	mw := &Middlewares{
		PractitionerFhirClient: &mockPractitionerClient{
			practitioners: []fhir_dto.Practitioner{
				{ID: "prac-1"},
			},
		},
		PatientFhirClient: &mockPatientClient{
			patients: []fhir_dto.Patient{
				{ID: "pat-1"},
			},
		},
	}

	ctx := context.WithValue(context.Background(), keyActiveRole, constvars.KonsulinRolePatient)
	role, id, err := mw.resolveFHIRIdentity(ctx, "user-123")

	assert.NoError(t, err)
	assert.Equal(t, constvars.KonsulinRolePatient, role)
	assert.Equal(t, "pat-1", id)
}

func TestResolveFHIRIdentity_ActiveRoleEmpty(t *testing.T) {
	// When activeRole is empty (not set), the default Practitioner-first behavior applies.
	mw := &Middlewares{
		PractitionerFhirClient: &mockPractitionerClient{
			practitioners: []fhir_dto.Practitioner{
				{ID: "prac-1"},
			},
		},
		PatientFhirClient: &mockPatientClient{
			patients: []fhir_dto.Patient{
				{ID: "pat-1"},
			},
		},
	}

	ctx := context.Background()
	role, id, err := mw.resolveFHIRIdentity(ctx, "user-123")

	assert.NoError(t, err)
	assert.Equal(t, constvars.KonsulinRolePractitioner, role)
	assert.Equal(t, "prac-1", id)
}

func TestResolveFHIRIdentity_ActiveRolePatient_NoPatientResource(t *testing.T) {
	// When activeRole is "Patient" but no Patient FHIR resource exists,
	// it should fall through to Practitioner lookup.
	mw := &Middlewares{
		PractitionerFhirClient: &mockPractitionerClient{
			practitioners: []fhir_dto.Practitioner{
				{ID: "prac-1"},
			},
		},
		PatientFhirClient: &mockPatientClient{
			patients: nil, // No patient resource
		},
	}

	ctx := context.WithValue(context.Background(), keyActiveRole, constvars.KonsulinRolePatient)
	role, id, err := mw.resolveFHIRIdentity(ctx, "user-123")

	assert.NoError(t, err)
	assert.Equal(t, constvars.KonsulinRolePractitioner, role)
	assert.Equal(t, "prac-1", id)
}
