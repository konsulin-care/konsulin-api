package organization

import (
	"context"
	"testing"

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
