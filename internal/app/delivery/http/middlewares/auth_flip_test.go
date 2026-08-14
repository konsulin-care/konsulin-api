package middlewares

import (
	"context"
	"testing"

	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/fhir_dto"
	"konsulin-service/internal/pkg/ownership"

	"github.com/stretchr/testify/assert"
)

// codingPractitionerRoleClient returns a configurable PractitionerRole list so
// tests can verify the FHIR-driven coding population in the ownership context.
type codingPractitionerRoleClient struct {
	roles []fhir_dto.PractitionerRole
}

func (*codingPractitionerRoleClient) DeletePractitionerRoleByID(context.Context, string) error {
	return nil
}

func (*codingPractitionerRoleClient) FindPractitionerRoleByOrganizationID(context.Context, string) ([]fhir_dto.PractitionerRole, error) {
	return nil, nil
}

func (*codingPractitionerRoleClient) FindPractitionerRoleByCustomRequest(context.Context, *requests.FindAllCliniciansByClinicID) ([]fhir_dto.PractitionerRole, error) {
	return nil, nil
}

func (c *codingPractitionerRoleClient) FindPractitionerRoleByPractitionerID(context.Context, string) ([]fhir_dto.PractitionerRole, error) {
	return c.roles, nil
}

func (*codingPractitionerRoleClient) FindPractitionerRoleByPractitionerIDAndOrganizationID(context.Context, string, string) ([]fhir_dto.PractitionerRole, error) {
	return nil, nil
}

func (*codingPractitionerRoleClient) CreatePractitionerRoles(context.Context, interface{}) error {
	return nil
}

func (*codingPractitionerRoleClient) CreatePractitionerRole(context.Context, *fhir_dto.PractitionerRole) (*fhir_dto.PractitionerRole, error) {
	return nil, nil
}

func (*codingPractitionerRoleClient) UpdatePractitionerRole(context.Context, *fhir_dto.PractitionerRole) (*fhir_dto.PractitionerRole, error) {
	return nil, nil
}

func (*codingPractitionerRoleClient) FindPractitionerRoleByPractitionerIDAndName(context.Context, *requests.FindClinicianByClinicianID) ([]fhir_dto.PractitionerRole, error) {
	return nil, nil
}

func (*codingPractitionerRoleClient) FindPractitionerRoleByID(context.Context, string) (*fhir_dto.PractitionerRole, error) {
	return nil, nil
}

func (*codingPractitionerRoleClient) Search(context.Context, contracts.PractitionerRoleSearchParams) ([]fhir_dto.PractitionerRole, error) {
	return nil, nil
}

func TestResolveUserRoles_ClinicAdminResolvesAsPractitioner(t *testing.T) {
	// Post-Phase-2: a Clinic Admin session resolves as a Practitioner identity
	// (the admin coding in the ownership context does the role gating).
	mw := &Middlewares{
		PractitionerFhirClient: &mockPractitionerClient{
			practitioners: []fhir_dto.Practitioner{{ID: "prac-1"}},
		},
		PatientFhirClient: &mockPatientClient{},
	}
	ctx := context.Background()

	role, id, err := mw.ResolveUserRoles(ctx, []string{constvars.KonsulinRoleClinicAdmin}, "admin-user-1")
	assert.NoError(t, err)
	assert.Equal(t, constvars.KonsulinRolePractitioner, role)
	assert.Equal(t, "prac-1", id)
}

func TestResolveUserRoles_ResearcherResolvesAsPractitioner(t *testing.T) {
	mw := &Middlewares{
		PractitionerFhirClient: &mockPractitionerClient{
			practitioners: []fhir_dto.Practitioner{{ID: "prac-1"}},
		},
		PatientFhirClient: &mockPatientClient{},
	}
	ctx := context.Background()

	role, id, err := mw.ResolveUserRoles(ctx, []string{constvars.KonsulinRoleResearcher}, "researcher-user-1")
	assert.NoError(t, err)
	assert.Equal(t, constvars.KonsulinRolePractitioner, role)
	assert.Equal(t, "prac-1", id)
}

func TestIsFHIRIdentityRole(t *testing.T) {
	for _, role := range []string{
		constvars.KonsulinRolePatient,
		constvars.KonsulinRolePractitioner,
		constvars.KonsulinRoleResearcher,
		constvars.KonsulinRoleClinicAdmin,
	} {
		assert.True(t, isFHIRIdentityRole(role), "%s should be a FHIR identity role", role)
	}
	for _, role := range []string{constvars.KonsulinRoleGuest, constvars.KonsulinRoleSuperadmin} {
		assert.False(t, isFHIRIdentityRole(role), "%s should not be a FHIR identity role", role)
	}
}

// emailPractitionerClient returns a Practitioner carrying a telecom email so
// tests can exercise the legacy email-based patient seeding path.
type emailPractitionerClient struct {
	mockPractitionerClient
}

func (*emailPractitionerClient) FindPractitionerByID(context.Context, string) (*fhir_dto.Practitioner, error) {
	return &fhir_dto.Practitioner{
		ID:      "prac-1",
		Telecom: []fhir_dto.ContactPoint{{System: fhir_dto.ContactPointSystemEmail, Value: "shared@example.com"}},
	}, nil
}

// emailOnlyPatientClient returns patients only via the email lookup, never via
// the SuperTokens identifier: it simulates legacy or directly-created records
// whose telecom email collides with a practitioner's email.
type emailOnlyPatientClient struct {
	mockPatientClient
}

func (*emailOnlyPatientClient) FindPatientByIdentifier(context.Context, string) ([]fhir_dto.Patient, error) {
	return nil, nil
}

func (*emailOnlyPatientClient) FindPatientByEmail(context.Context, string) ([]fhir_dto.Patient, error) {
	return []fhir_dto.Patient{{ID: "pat-shared-email"}}, nil
}

func TestOwnershipContextFromRoles_HoldsPractitionerRoleOnlyForGenuinePractitioner(t *testing.T) {
	// HoldsPractitionerRole marks a session that genuinely holds the Practitioner
	// role; Researcher and Clinic Admin set HasPractitionerRole (they resolve as
	// practitioners) but must not be treated as relationship-based practitioners.
	genuine := ownershipContextFromRoles([]string{constvars.KonsulinRolePractitioner}, constvars.KonsulinRolePractitioner, "prac-1")
	assert.True(t, genuine.HoldsPractitionerRole)
	assert.True(t, genuine.HasPractitionerRole)

	researcher := ownershipContextFromRoles([]string{constvars.KonsulinRoleResearcher}, constvars.KonsulinRolePractitioner, "prac-1")
	assert.False(t, researcher.HoldsPractitionerRole, "researcher must not hold the genuine practitioner flag")
	assert.True(t, researcher.HasPractitionerRole)
	assert.True(t, researcher.HoldsCoding(ownership.CodingResearcher))

	admin := ownershipContextFromRoles([]string{constvars.KonsulinRoleClinicAdmin}, constvars.KonsulinRolePractitioner, "prac-1")
	assert.False(t, admin.HoldsPractitionerRole, "clinic admin must not hold the genuine practitioner flag")
	assert.True(t, admin.HasPractitionerRole)
	assert.True(t, admin.HoldsCoding(ownership.CodingClinicAdmin))

	patient := ownershipContextFromRoles([]string{constvars.KonsulinRolePatient}, constvars.KonsulinRolePatient, "pat-1")
	assert.False(t, patient.HoldsPractitionerRole)
}

func TestBuildOwnershipContext_ResearcherGetsNoPatientSeeding(t *testing.T) {
	// A Researcher session resolves as a Practitioner identity but must not
	// inherit the practitioner relationship seeding: no PatientIDs are seeded,
	// even when a patient record exists for the same SuperTokens uid.
	mw := &Middlewares{
		PatientFhirClient:          &mockPatientClient{patients: []fhir_dto.Patient{{ID: "pat-1"}}},
		PractitionerFhirClient:     &mockPractitionerClient{practitioners: []fhir_dto.Practitioner{{ID: "prac-1"}}},
		PractitionerRoleFhirClient: &stubPractitionerRoleClient{},
	}
	ctx := context.WithValue(context.Background(), keyUID, "user-123")

	oc := mw.buildOwnershipContext(ctx, []string{constvars.KonsulinRoleResearcher}, constvars.KonsulinRolePractitioner, "prac-1")

	assert.Contains(t, oc.PractitionerIDs, "prac-1")
	assert.Empty(t, oc.PatientIDs, "researcher session must not seed patient identities")
	assert.True(t, oc.HoldsCoding(ownership.CodingResearcher))
}

func TestBuildOwnershipContext_GenuinePractitionerSeedsOwnPatientBySupertoken(t *testing.T) {
	// The one legitimate related-patient case: a practitioner who is also a
	// patient (dual identity) reads their own Patient record, matched by the
	// SuperTokens identifier — not by email.
	mw := &Middlewares{
		PatientFhirClient:          &mockPatientClient{patients: []fhir_dto.Patient{{ID: "pat-1"}}},
		PractitionerFhirClient:     &mockPractitionerClient{practitioners: []fhir_dto.Practitioner{{ID: "prac-1"}}},
		PractitionerRoleFhirClient: &stubPractitionerRoleClient{},
	}
	ctx := context.WithValue(context.Background(), keyUID, "user-123")

	oc := mw.buildOwnershipContext(ctx, []string{constvars.KonsulinRolePractitioner}, constvars.KonsulinRolePractitioner, "prac-1")

	assert.Contains(t, oc.PatientIDs, "pat-1")
}

func TestBuildOwnershipContext_EmailSharedPatientsNotSeeded(t *testing.T) {
	// A practitioner whose FHIR record shares an email with an unrelated
	// patient must not gain access: seeding is by the SuperTokens identifier,
	// so an email-only match seeds nothing.
	mw := &Middlewares{
		PatientFhirClient:          &emailOnlyPatientClient{},
		PractitionerFhirClient:     &emailPractitionerClient{},
		PractitionerRoleFhirClient: &stubPractitionerRoleClient{},
	}
	ctx := context.WithValue(context.Background(), keyUID, "user-123")

	oc := mw.buildOwnershipContext(ctx, []string{constvars.KonsulinRolePractitioner}, constvars.KonsulinRolePractitioner, "prac-1")

	assert.Empty(t, oc.PatientIDs, "email-shared patients must not be seeded")
}

func TestBuildOwnershipContext_RoleCodingsFromSession(t *testing.T) {
	// Session roles map directly to the Phase 1 practitioner-role codings.
	mw := &Middlewares{
		PractitionerFhirClient:     &mockPractitionerClient{},
		PatientFhirClient:          &mockPatientClient{},
		PractitionerRoleFhirClient: &stubPractitionerRoleClient{},
	}
	ctx := context.Background()

	oc := mw.buildOwnershipContext(ctx,
		[]string{constvars.KonsulinRolePatient, constvars.KonsulinRolePractitioner, constvars.KonsulinRoleResearcher, constvars.KonsulinRoleClinicAdmin},
		constvars.KonsulinRolePractitioner, "prac-1")

	assert.True(t, oc.HasPatientRole)
	assert.True(t, oc.HasPractitionerRole)
	assert.True(t, oc.HoldsCoding(ownership.CodingResearcher))
	assert.True(t, oc.HoldsCoding(ownership.CodingClinicAdmin))
	assert.Contains(t, oc.PractitionerIDs, "prac-1")
}

func TestBuildOwnershipContext_CodingsFromPractitionerRoleResources(t *testing.T) {
	// Codings are also populated from the practitioner's FHIR PractitionerRole
	// resources (system|code pairs), so a practitioner holding a role resource
	// with the researcher coding is treated as a researcher.
	mw := &Middlewares{
		PractitionerFhirClient: &mockPractitionerClient{},
		PatientFhirClient:      &mockPatientClient{},
		PractitionerRoleFhirClient: &codingPractitionerRoleClient{roles: []fhir_dto.PractitionerRole{{
			ID: "role-1",
			Code: []fhir_dto.CodeableConcept{{
				Coding: []fhir_dto.Coding{{
					System: constvars.FhirPractitionerRoleSystemHL7,
					Code:   constvars.FhirPractitionerRoleCodeResearcher,
				}},
			}},
		}}},
	}
	ctx := context.Background()

	oc := mw.buildOwnershipContext(ctx, []string{constvars.KonsulinRolePractitioner}, constvars.KonsulinRolePractitioner, "prac-1")

	assert.Contains(t, oc.PractitionerRoleIDs, "role-1")
	assert.True(t, oc.HoldsCoding(ownership.CodingResearcher))
}

func TestOwnsResource_ResearcherPlanDefinitionPUTAllowed(t *testing.T) {
	// Researcher writes PlanDefinition (Public rule, no WriteRefs) are allowed;
	// Casbin gates the path, ownership imposes no extra constraint.
	got := ownsResource(context.Background(), "prac-1", "/fhir/PlanDefinition", constvars.KonsulinRoleResearcher, constvars.MethodPut, rbacClients{},
		[]byte(`{"resourceType":"PlanDefinition","id":"pd-1"}`))
	assert.True(t, got)
}

func TestOwnsResource_ClinicAdminPractitionerRolePUTBypass(t *testing.T) {
	// A clinic admin manages PractitionerRole resources for other practitioners
	// (WriteBypassCodes on the rule); the PUT must not require practitioner==self.
	got := ownsResource(context.Background(), "prac-1", "/fhir/PractitionerRole", constvars.KonsulinRoleClinicAdmin, constvars.MethodPut, rbacClients{},
		[]byte(`{"resourceType":"PractitionerRole","practitioner":{"reference":"Practitioner/prac-2"}}`))
	assert.True(t, got)
}

func TestOwnsResource_PractitionerPractitionerRolePUTSelfRequired(t *testing.T) {
	// A plain practitioner can only PUT a PractitionerRole bound to themselves.
	got := ownsResource(context.Background(), "prac-1", "/fhir/PractitionerRole", constvars.KonsulinRolePractitioner, constvars.MethodPut, rbacClients{},
		[]byte(`{"resourceType":"PractitionerRole","practitioner":{"reference":"Practitioner/prac-2"}}`))
	assert.False(t, got)
}

func TestOwnsResource_ClinicAdminSchedulePOSTAllowed(t *testing.T) {
	// Clinic Admin creates schedules for their clinicians; the ownership engine
	// imposes no actor==self constraint (Schedule rule has no WriteRefs).
	got := ownsResource(context.Background(), "prac-1", "/fhir/Schedule", constvars.KonsulinRoleClinicAdmin, constvars.MethodPost, rbacClients{},
		[]byte(`{"resourceType":"Schedule","actor":[{"reference":"Practitioner/prac-2"}]}`))
	assert.True(t, got)
}

func TestValidatePostRequestBody_ClinicAdminScheduleAllowed(t *testing.T) {
	// Post-flip the clinic admin resolves as a Practitioner, but the Schedule
	// rule carries no WriteRefs, so body validation stays permissive.
	mw := &Middlewares{}
	ctx := context.WithValue(context.Background(), keyRoles, []string{constvars.KonsulinRoleClinicAdmin})
	err := mw.validatePostRequestBody(ctx,
		[]byte(`{"resourceType":"Schedule","actor":[{"reference":"Practitioner/prac-2"}]}`),
		constvars.KonsulinRolePractitioner, "prac-1")
	assert.NoError(t, err)
}
