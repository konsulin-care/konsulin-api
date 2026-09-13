package middlewares

import (
	"context"
	"testing"

	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/fhir_dto"

	"github.com/stretchr/testify/assert"
)

// stubPractitionerRoleClient returns empty results for every PractitionerRole
// query; used where role resolution runs but its results are not under test.
type stubPractitionerRoleClient struct{}

func (*stubPractitionerRoleClient) DeletePractitionerRoleByID(context.Context, string) error {
	return nil
}
func (*stubPractitionerRoleClient) FindPractitionerRoleByOrganizationID(context.Context, string) ([]fhir_dto.PractitionerRole, error) {
	return nil, nil
}
func (*stubPractitionerRoleClient) FindPractitionerRoleByCustomRequest(context.Context, *requests.FindAllCliniciansByClinicID) ([]fhir_dto.PractitionerRole, error) {
	return nil, nil
}
func (*stubPractitionerRoleClient) FindPractitionerRoleByPractitionerID(context.Context, string) ([]fhir_dto.PractitionerRole, error) {
	return nil, nil
}
func (*stubPractitionerRoleClient) FindPractitionerRoleByPractitionerIDAndOrganizationID(context.Context, string, string) ([]fhir_dto.PractitionerRole, error) {
	return nil, nil
}
func (*stubPractitionerRoleClient) CreatePractitionerRoles(context.Context, interface{}) error {
	return nil
}
func (*stubPractitionerRoleClient) CreatePractitionerRole(context.Context, *fhir_dto.PractitionerRole) (*fhir_dto.PractitionerRole, error) {
	return nil, nil
}
func (*stubPractitionerRoleClient) UpdatePractitionerRole(context.Context, *fhir_dto.PractitionerRole) (*fhir_dto.PractitionerRole, error) {
	return nil, nil
}
func (*stubPractitionerRoleClient) FindPractitionerRoleByPractitionerIDAndName(context.Context, *requests.FindClinicianByClinicianID) ([]fhir_dto.PractitionerRole, error) {
	return nil, nil
}
func (*stubPractitionerRoleClient) FindPractitionerRoleByID(context.Context, string) (*fhir_dto.PractitionerRole, error) {
	return nil, nil
}
func (*stubPractitionerRoleClient) Search(context.Context, contracts.PractitionerRoleSearchParams) ([]fhir_dto.PractitionerRole, error) {
	return nil, nil
}

func TestBuildOwnershipContext_DualRoleActivePatientSeedsBothIDs(t *testing.T) {
	// A dual-role user with active role Patient owns both identities: the
	// ownership context must contain both the Patient ID and the Practitioner ID.
	prac := &countingPractitionerClient{mockPractitionerClient: mockPractitionerClient{
		practitioners: []fhir_dto.Practitioner{{ID: "prac-1"}},
	}}
	mw := &Middlewares{
		PatientFhirClient:      &mockPatientClient{patients: []fhir_dto.Patient{{ID: "pat-1"}}},
		PractitionerFhirClient: prac,
	}
	ctx := context.WithValue(context.Background(), keyUID, "user-123")
	roles := []string{constvars.KonsulinRolePatient, constvars.KonsulinRolePractitioner}

	oc := mw.buildOwnershipContext(ctx, roles, constvars.KonsulinRolePatient, "pat-1")

	assert.Contains(t, oc.PatientIDs, "pat-1")
	assert.Contains(t, oc.PractitionerIDs, "prac-1")
	// Exactly one secondary lookup for the whole context.
	assert.Equal(t, 1, prac.byIdentifierCalls)
}

func TestBuildOwnershipContext_DualRoleActivePractitionerSeedsBothIDs(t *testing.T) {
	// Symmetric case: active role Practitioner also owns the Patient identity.
	prac := &countingPractitionerClient{mockPractitionerClient: mockPractitionerClient{
		practitioners: []fhir_dto.Practitioner{{ID: "prac-1"}},
	}}
	mw := &Middlewares{
		PatientFhirClient:          &mockPatientClient{patients: []fhir_dto.Patient{{ID: "pat-1"}}},
		PractitionerFhirClient:     prac,
		PractitionerRoleFhirClient: &stubPractitionerRoleClient{},
	}
	ctx := context.WithValue(context.Background(), keyUID, "user-123")
	roles := []string{constvars.KonsulinRolePatient, constvars.KonsulinRolePractitioner}

	oc := mw.buildOwnershipContext(ctx, roles, constvars.KonsulinRolePractitioner, "prac-1")

	assert.Contains(t, oc.PractitionerIDs, "prac-1")
	assert.Contains(t, oc.PatientIDs, "pat-1")
}

func TestBuildOwnershipContext_SingleRoleNoExtraLookup(t *testing.T) {
	// A Patient-only session must not pay for any practitioner lookup.
	prac := &countingPractitionerClient{mockPractitionerClient: mockPractitionerClient{
		practitioners: []fhir_dto.Practitioner{{ID: "prac-1"}},
	}}
	mw := &Middlewares{
		PatientFhirClient:      &mockPatientClient{patients: []fhir_dto.Patient{{ID: "pat-1"}}},
		PractitionerFhirClient: prac,
	}
	ctx := context.WithValue(context.Background(), keyUID, "user-123")

	oc := mw.buildOwnershipContext(ctx, []string{constvars.KonsulinRolePatient}, constvars.KonsulinRolePatient, "pat-1")

	assert.Contains(t, oc.PatientIDs, "pat-1")
	assert.Empty(t, oc.PractitionerIDs)
	assert.Zero(t, prac.byIdentifierCalls)
}

func TestFilterSingleResourceByOwnership_DualRoleActivePractitionerReadsOwnPatient(t *testing.T) {
	// A dual-role user with active role Practitioner can GET their own Patient
	// resource: the response filter must not strip it.
	mw := &Middlewares{
		PatientFhirClient:          &mockPatientClient{patients: []fhir_dto.Patient{{ID: "pat-1"}}},
		PractitionerFhirClient:     &mockPractitionerClient{practitioners: []fhir_dto.Practitioner{{ID: "prac-1"}}},
		PractitionerRoleFhirClient: &stubPractitionerRoleClient{},
	}
	ctx := context.WithValue(context.Background(), keyUID, "user-123")
	body := []byte(`{"resourceType":"Patient","id":"pat-1"}`)
	roles := []string{constvars.KonsulinRolePatient, constvars.KonsulinRolePractitioner}

	filtered, allowed, err := mw.filterSingleResourceByOwnership(ctx, body, roles, constvars.KonsulinRolePractitioner, "prac-1")

	assert.NoError(t, err)
	assert.True(t, allowed)
	assert.Equal(t, body, filtered)
}

func TestFilterSingleResourceByOwnership_DualRoleActivePractitionerDeniedOtherPatient(t *testing.T) {
	// The dual-identity seeding must not widen access: another patient's
	// resource is still filtered out.
	mw := &Middlewares{
		PatientFhirClient:          &mockPatientClient{patients: []fhir_dto.Patient{{ID: "pat-1"}}},
		PractitionerFhirClient:     &mockPractitionerClient{practitioners: []fhir_dto.Practitioner{{ID: "prac-1"}}},
		PractitionerRoleFhirClient: &stubPractitionerRoleClient{},
	}
	ctx := context.WithValue(context.Background(), keyUID, "user-123")
	body := []byte(`{"resourceType":"Patient","id":"other-pat"}`)
	roles := []string{constvars.KonsulinRolePatient, constvars.KonsulinRolePractitioner}

	_, allowed, err := mw.filterSingleResourceByOwnership(ctx, body, roles, constvars.KonsulinRolePractitioner, "prac-1")

	assert.NoError(t, err)
	assert.False(t, allowed)
}

func TestFilterSingleResourceByOwnership_DualRoleActivePatientReadsOwnPractitioner(t *testing.T) {
	// Active role Patient can still GET their own Practitioner resource
	// (Practitioner is a public resource type; this guards the read path).
	mw := &Middlewares{
		PatientFhirClient:      &mockPatientClient{patients: []fhir_dto.Patient{{ID: "pat-1"}}},
		PractitionerFhirClient: &mockPractitionerClient{practitioners: []fhir_dto.Practitioner{{ID: "prac-1"}}},
	}
	ctx := context.WithValue(context.Background(), keyUID, "user-123")
	body := []byte(`{"resourceType":"Practitioner","id":"prac-1"}`)
	roles := []string{constvars.KonsulinRolePatient, constvars.KonsulinRolePractitioner}

	filtered, allowed, err := mw.filterSingleResourceByOwnership(ctx, body, roles, constvars.KonsulinRolePatient, "pat-1")

	assert.NoError(t, err)
	assert.True(t, allowed)
	assert.Equal(t, body, filtered)
}
