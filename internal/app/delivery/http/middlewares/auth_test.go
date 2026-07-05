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

func (m *mockPatientClient) FindPatientByIdentifier(_ context.Context, _ string) ([]fhir_dto.Patient, error) {
	return m.patients, m.err
}

func (*mockPatientClient) FindPatientByID(_ context.Context, _ string) (*fhir_dto.Patient, error) {
	return nil, nil
}
func (*mockPatientClient) FindPatientByEmail(_ context.Context, _ string) ([]fhir_dto.Patient, error) {
	return nil, nil
}
func (*mockPatientClient) FindPatientByPhone(_ context.Context, _ string) ([]fhir_dto.Patient, error) {
	return nil, nil
}
func (*mockPatientClient) CreatePatient(_ context.Context, _ *fhir_dto.Patient) (*fhir_dto.Patient, error) {
	return nil, nil
}
func (*mockPatientClient) UpdatePatient(_ context.Context, _ *fhir_dto.Patient) (*fhir_dto.Patient, error) {
	return nil, nil
}
func (*mockPatientClient) PatchPatient(_ context.Context, _ *fhir_dto.Patient) (*fhir_dto.Patient, error) {
	return nil, nil
}

// mockPractitionerClient returns configurable results for PractitionerFhirClient.
type mockPractitionerClient struct {
	practitioners []fhir_dto.Practitioner
	err           error
}

func (m *mockPractitionerClient) FindPractitionerByIdentifier(_ context.Context, _, _ string) ([]fhir_dto.Practitioner, error) {
	return m.practitioners, m.err
}

func (*mockPractitionerClient) FindPractitionerByID(_ context.Context, _ string) (*fhir_dto.Practitioner, error) {
	return nil, nil
}
func (*mockPractitionerClient) FindPractitionerByEmail(_ context.Context, _ string) ([]fhir_dto.Practitioner, error) {
	return nil, nil
}
func (*mockPractitionerClient) FindPractitionerByPhone(_ context.Context, _ string) ([]fhir_dto.Practitioner, error) {
	return nil, nil
}
func (*mockPractitionerClient) CreatePractitioner(_ context.Context, _ *fhir_dto.Practitioner) (*fhir_dto.Practitioner, error) {
	return nil, nil
}
func (*mockPractitionerClient) UpdatePractitioner(_ context.Context, _ *fhir_dto.Practitioner) (*fhir_dto.Practitioner, error) {
	return nil, nil
}
func (*mockPractitionerClient) PatchPractitioner(_ context.Context, _ *fhir_dto.Practitioner) (*fhir_dto.Practitioner, error) {
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

func TestNeedsFHIRResolution(t *testing.T) {
	tests := []struct {
		name  string
		roles []string
		want  bool
	}{
		{"Patient role needs resolution", []string{constvars.KonsulinRolePatient}, true},
		{"Practitioner role needs resolution", []string{constvars.KonsulinRolePractitioner}, true},
		{"Clinic Admin does not need resolution", []string{constvars.KonsulinRoleClinicAdmin}, false},
		{"Researcher does not need resolution", []string{constvars.KonsulinRoleResearcher}, false},
		{"Guest does not need resolution", []string{constvars.KonsulinRoleGuest}, false},
		{"Multiple roles with Patient needs resolution", []string{constvars.KonsulinRoleResearcher, constvars.KonsulinRolePatient}, true},
		{"Multiple roles without Patient/Practitioner", []string{constvars.KonsulinRoleClinicAdmin, constvars.KonsulinRoleResearcher}, false},
		{"Empty roles does not need resolution", []string{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := needsFHIRResolution(tt.roles)
			if got != tt.want {
				t.Errorf("needsFHIRResolution(%v) = %v, want %v", tt.roles, got, tt.want)
			}
		})
	}
}

func TestResolveFHIRIdentity_ActiveRolePatient_MultiMatch(t *testing.T) {
	// When activeRole is "Patient" and multiple Patient FHIR resources exist,
	// resolveFHIRIdentity should return an error mirroring the Practitioner multi-match guard.
	mw := &Middlewares{
		PractitionerFhirClient: &mockPractitionerClient{},
		PatientFhirClient: &mockPatientClient{
			patients: []fhir_dto.Patient{
				{ID: "pat-1"},
				{ID: "pat-2"},
			},
		},
	}

	ctx := context.WithValue(context.Background(), keyActiveRole, constvars.KonsulinRolePatient)
	role, id, err := mw.resolveFHIRIdentity(ctx, "user-123")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "multiple Patient resources")
	assert.Empty(t, role)
	assert.Empty(t, id)
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
