package organization

import (
	"context"
	"errors"
	"testing"

	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/fhir_dto"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
)

type mockPractitionerFhirClient struct {
	mock.Mock
}

func (m *mockPractitionerFhirClient) CreatePractitioner(ctx context.Context, req *fhir_dto.Practitioner) (*fhir_dto.Practitioner, error) {
	args := m.Called(ctx, req)
	if v := args.Get(0); v != nil {
		return v.(*fhir_dto.Practitioner), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockPractitionerFhirClient) UpdatePractitioner(ctx context.Context, req *fhir_dto.Practitioner) (*fhir_dto.Practitioner, error) { // NOSONAR:go:S4144 testify mock idiom
	args := m.Called(ctx, req)
	if v := args.Get(0); v != nil {
		return v.(*fhir_dto.Practitioner), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockPractitionerFhirClient) PatchPractitioner(ctx context.Context, req *fhir_dto.Practitioner) (*fhir_dto.Practitioner, error) { // NOSONAR:go:S4144 testify mock idiom
	args := m.Called(ctx, req)
	if v := args.Get(0); v != nil {
		return v.(*fhir_dto.Practitioner), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockPractitionerFhirClient) FindPractitionerByID(ctx context.Context, practitionerID string) (*fhir_dto.Practitioner, error) {
	args := m.Called(ctx, practitionerID)
	if v := args.Get(0); v != nil {
		return v.(*fhir_dto.Practitioner), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockPractitionerFhirClient) FindPractitionerByIdentifier(ctx context.Context, system, value string) ([]fhir_dto.Practitioner, error) {
	args := m.Called(ctx, system, value)
	if v := args.Get(0); v != nil {
		return v.([]fhir_dto.Practitioner), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockPractitionerFhirClient) FindPractitionerByEmail(ctx context.Context, email string) ([]fhir_dto.Practitioner, error) {
	args := m.Called(ctx, email)
	if v := args.Get(0); v != nil {
		return v.([]fhir_dto.Practitioner), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockPractitionerFhirClient) FindPractitionerByPhone(ctx context.Context, phone string) ([]fhir_dto.Practitioner, error) {
	args := m.Called(ctx, phone)
	if v := args.Get(0); v != nil {
		return v.([]fhir_dto.Practitioner), args.Error(1)
	}
	return nil, args.Error(1)
}

type mockPractitionerRoleFhirClient struct {
	mock.Mock
}

func (m *mockPractitionerRoleFhirClient) DeletePractitionerRoleByID(ctx context.Context, practitionerRoleID string) error {
	return m.Called(ctx, practitionerRoleID).Error(0)
}

func (m *mockPractitionerRoleFhirClient) FindPractitionerRoleByOrganizationID(ctx context.Context, organizationID string) ([]fhir_dto.PractitionerRole, error) {
	args := m.Called(ctx, organizationID)
	if v := args.Get(0); v != nil {
		return v.([]fhir_dto.PractitionerRole), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockPractitionerRoleFhirClient) FindPractitionerRoleByCustomRequest(ctx context.Context, request *requests.FindAllCliniciansByClinicID) ([]fhir_dto.PractitionerRole, error) {
	args := m.Called(ctx, request)
	if v := args.Get(0); v != nil {
		return v.([]fhir_dto.PractitionerRole), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockPractitionerRoleFhirClient) FindPractitionerRoleByPractitionerID(ctx context.Context, practitionerID string) ([]fhir_dto.PractitionerRole, error) {
	args := m.Called(ctx, practitionerID)
	if v := args.Get(0); v != nil {
		return v.([]fhir_dto.PractitionerRole), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockPractitionerRoleFhirClient) FindPractitionerRoleByPractitionerIDAndOrganizationID(ctx context.Context, practitionerID, organizationID string) ([]fhir_dto.PractitionerRole, error) {
	args := m.Called(ctx, practitionerID, organizationID)
	if v := args.Get(0); v != nil {
		return v.([]fhir_dto.PractitionerRole), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockPractitionerRoleFhirClient) CreatePractitionerRoles(ctx context.Context, request interface{}) error {
	return m.Called(ctx, request).Error(0)
}

func (m *mockPractitionerRoleFhirClient) CreatePractitionerRole(ctx context.Context, request *fhir_dto.PractitionerRole) (*fhir_dto.PractitionerRole, error) {
	args := m.Called(ctx, request)
	if v := args.Get(0); v != nil {
		return v.(*fhir_dto.PractitionerRole), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockPractitionerRoleFhirClient) UpdatePractitionerRole(ctx context.Context, request *fhir_dto.PractitionerRole) (*fhir_dto.PractitionerRole, error) { // NOSONAR:go:S4144 testify mock idiom
	args := m.Called(ctx, request)
	if v := args.Get(0); v != nil {
		return v.(*fhir_dto.PractitionerRole), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockPractitionerRoleFhirClient) FindPractitionerRoleByPractitionerIDAndName(ctx context.Context, request *requests.FindClinicianByClinicianID) ([]fhir_dto.PractitionerRole, error) {
	args := m.Called(ctx, request)
	if v := args.Get(0); v != nil {
		return v.([]fhir_dto.PractitionerRole), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockPractitionerRoleFhirClient) FindPractitionerRoleByID(ctx context.Context, practitionerRoleID string) (*fhir_dto.PractitionerRole, error) {
	args := m.Called(ctx, practitionerRoleID)
	if v := args.Get(0); v != nil {
		return v.(*fhir_dto.PractitionerRole), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockPractitionerRoleFhirClient) Search(ctx context.Context, params contracts.PractitionerRoleSearchParams) ([]fhir_dto.PractitionerRole, error) {
	args := m.Called(ctx, params)
	if v := args.Get(0); v != nil {
		return v.([]fhir_dto.PractitionerRole), args.Error(1)
	}
	return nil, args.Error(1)
}

func orgAdminStaffRole(orgRef string) fhir_dto.PractitionerRole {
	return fhir_dto.PractitionerRole{
		Code: []fhir_dto.CodeableConcept{{
			Coding: []fhir_dto.Coding{{Code: constvars.FhirPractitionerRoleCodeAdministrativeStaff}},
		}},
		Organization: fhir_dto.Reference{Reference: orgRef},
	}
}

// TestEnsureClinicAdminManagesOrganization verifies the org scope check resolves
// the admin's organization from their admin-coded PractitionerRole.
func TestEnsureClinicAdminManagesOrganization(t *testing.T) {
	t.Run("admin managing the organization passes", func(t *testing.T) {
		mockPrac := new(mockPractitionerFhirClient)
		mockPR := new(mockPractitionerRoleFhirClient)
		uc := &Usecase{practitionerClient: mockPrac, practitionerRoleClient: mockPR, log: zap.NewNop()}

		mockPrac.On("FindPractitionerByIdentifier", mock.Anything, constvars.FhirSupertokenSystemIdentifier, "st-123").
			Return([]fhir_dto.Practitioner{{ID: "prac-1"}}, nil)
		mockPR.On("FindPractitionerRoleByPractitionerID", mock.Anything, "prac-1").
			Return([]fhir_dto.PractitionerRole{orgAdminStaffRole("Organization/org-1")}, nil)

		err := uc.ensureClinicAdminManagesOrganization(context.Background(), "st-123", "org-1")
		assert.NoError(t, err)
		mockPrac.AssertExpectations(t)
		mockPR.AssertExpectations(t)
	})

	t.Run("admin not managing the organization rejected", func(t *testing.T) {
		mockPrac := new(mockPractitionerFhirClient)
		mockPR := new(mockPractitionerRoleFhirClient)
		uc := &Usecase{practitionerClient: mockPrac, practitionerRoleClient: mockPR, log: zap.NewNop()}

		mockPrac.On("FindPractitionerByIdentifier", mock.Anything, constvars.FhirSupertokenSystemIdentifier, "st-123").
			Return([]fhir_dto.Practitioner{{ID: "prac-1"}}, nil)
		mockPR.On("FindPractitionerRoleByPractitionerID", mock.Anything, "prac-1").
			Return([]fhir_dto.PractitionerRole{orgAdminStaffRole("Organization/org-2")}, nil)

		err := uc.ensureClinicAdminManagesOrganization(context.Background(), "st-123", "org-1")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "does not manage")
	})

	t.Run("admin without org-scoped admin role rejected", func(t *testing.T) {
		mockPrac := new(mockPractitionerFhirClient)
		mockPR := new(mockPractitionerRoleFhirClient)
		uc := &Usecase{practitionerClient: mockPrac, practitionerRoleClient: mockPR, log: zap.NewNop()}

		mockPrac.On("FindPractitionerByIdentifier", mock.Anything, constvars.FhirSupertokenSystemIdentifier, "st-123").
			Return([]fhir_dto.Practitioner{{ID: "prac-1"}}, nil)
		mockPR.On("FindPractitionerRoleByPractitionerID", mock.Anything, "prac-1").
			Return([]fhir_dto.PractitionerRole{{Organization: fhir_dto.Reference{Reference: "Organization/org-1"}}}, nil)

		err := uc.ensureClinicAdminManagesOrganization(context.Background(), "st-123", "org-1")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no org-scoped admin role")
	})

	t.Run("practitioner not found rejected", func(t *testing.T) {
		mockPrac := new(mockPractitionerFhirClient)
		mockPR := new(mockPractitionerRoleFhirClient)
		uc := &Usecase{practitionerClient: mockPrac, practitionerRoleClient: mockPR, log: zap.NewNop()}

		mockPrac.On("FindPractitionerByIdentifier", mock.Anything, constvars.FhirSupertokenSystemIdentifier, "st-123").
			Return([]fhir_dto.Practitioner{}, nil)

		err := uc.ensureClinicAdminManagesOrganization(context.Background(), "st-123", "org-1")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no practitioner found")
	})
}

type mockOrganizationFhirClient struct {
	mock.Mock
}

func (m *mockOrganizationFhirClient) FindAll(ctx context.Context, organizationName, fetchType string, page, pageSize int) ([]fhir_dto.Organization, int, error) {
	args := m.Called(ctx, organizationName, fetchType, page, pageSize)
	if v := args.Get(0); v != nil {
		return v.([]fhir_dto.Organization), args.Int(1), args.Error(2)
	}
	return nil, args.Int(1), args.Error(2)
}

func (m *mockOrganizationFhirClient) FindOrganizationByID(ctx context.Context, organizationID string) (*fhir_dto.Organization, error) {
	args := m.Called(ctx, organizationID)
	if v := args.Get(0); v != nil {
		return v.(*fhir_dto.Organization), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockOrganizationFhirClient) Update(ctx context.Context, organization fhir_dto.Organization) (*fhir_dto.Organization, error) {
	args := m.Called(ctx, organization)
	if v := args.Get(0); v != nil {
		return v.(*fhir_dto.Organization), args.Error(1)
	}
	return nil, args.Error(1)
}

// testOrgConfig builds an InternalConfig carrying only the archetype bootstrap
// settings: APP_ORGANIZATION_NAME and APP_PARTY_ID.
func testOrgConfig(orgName, partyID string) *config.InternalConfig {
	return &config.InternalConfig{
		App:     config.App{OrganizationName: orgName},
		Gateway: config.AppGateway{PartyID: partyID},
	}
}

// TestInitializeKonsulinOrganizationResource pins the archetype bootstrap
// (BK-PARTYID-001, BK-AUTH-002): initialization ensures Organization/archetype
// with the configured name and the urn:konsulin:party identifier carrying the
// party ID, updating an existing resource in place without duplicating identifiers.
func TestInitializeKonsulinOrganizationResource(t *testing.T) {
	t.Run("initialization creates archetype with configured name and party identifier", func(t *testing.T) {
		mockOrg := new(mockOrganizationFhirClient)
		uc := &Usecase{
			organizationClient: mockOrg,
			config:             testOrgConfig("Konsulin Clinic", "pty-1"),
			log:                zap.NewNop(),
		}

		mockOrg.On("FindOrganizationByID", mock.Anything, constvars.KonsulinOrganizationResourceID).
			Return(nil, errors.New("not found"))
		var got fhir_dto.Organization
		mockOrg.On("Update", mock.Anything, mock.Anything).
			Run(func(args mock.Arguments) { got = args.Get(1).(fhir_dto.Organization) }).
			Return(nil, nil)

		err := uc.InitializeKonsulinOrganizationResource(context.Background())
		assert.NoError(t, err)

		assert.Equal(t, "archetype", got.ID)
		assert.Equal(t, constvars.ResourceOrganization, got.ResourceType)
		assert.Equal(t, "Konsulin Clinic", got.Name)
		assert.True(t, got.Active)
		assert.Len(t, got.Identifier, 1)
		assert.Equal(t, "urn:konsulin:party", got.Identifier[0].System)
		assert.Equal(t, "pty-1", got.Identifier[0].Value)
		mockOrg.AssertExpectations(t)
	})

	t.Run("existing archetype without party identifier updated in place without duplicates", func(t *testing.T) {
		mockOrg := new(mockOrganizationFhirClient)
		uc := &Usecase{
			organizationClient: mockOrg,
			config:             testOrgConfig("Konsulin Clinic", "pty-1"),
			log:                zap.NewNop(),
		}

		existing := &fhir_dto.Organization{
			ResourceType: constvars.ResourceOrganization,
			ID:           "archetype",
			Active:       true,
			Name:         "Old Name",
			Identifier:   []fhir_dto.Identifier{{System: "urn:other", Value: "other-1"}},
		}
		mockOrg.On("FindOrganizationByID", mock.Anything, constvars.KonsulinOrganizationResourceID).
			Return(existing, nil)
		var got fhir_dto.Organization
		mockOrg.On("Update", mock.Anything, mock.Anything).
			Run(func(args mock.Arguments) { got = args.Get(1).(fhir_dto.Organization) }).
			Return(nil, nil)

		err := uc.InitializeKonsulinOrganizationResource(context.Background())
		assert.NoError(t, err)

		assert.Equal(t, "archetype", got.ID, "resource updated in place")
		assert.Equal(t, "Konsulin Clinic", got.Name)
		partyCount := 0
		for _, id := range got.Identifier {
			if id.System == "urn:konsulin:party" {
				partyCount++
				assert.Equal(t, "pty-1", id.Value)
			}
		}
		assert.Equal(t, 1, partyCount, "exactly one party identifier, no duplicates")
		mockOrg.AssertExpectations(t)
	})
}
