package webhook

import (
	"context"
	"testing"

	"konsulin-service/internal/pkg/constvars"
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

func (m *mockPractitionerFhirClient) UpdatePractitioner(ctx context.Context, req *fhir_dto.Practitioner) (*fhir_dto.Practitioner, error) {
	args := m.Called(ctx, req)
	if v := args.Get(0); v != nil {
		return v.(*fhir_dto.Practitioner), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockPractitionerFhirClient) PatchPractitioner(ctx context.Context, req *fhir_dto.Practitioner) (*fhir_dto.Practitioner, error) {
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

// TestValidateClinicAdminContact verifies clinic admin contact validation now
// resolves the Practitioner by supertoken identifier instead of Person.
func TestValidateClinicAdminContact(t *testing.T) {
	adminPrac := fhir_dto.Practitioner{
		Identifier: []fhir_dto.Identifier{
			{System: constvars.FhirSupertokenSystemIdentifier, Value: "st-123"},
			{System: constvars.KonsulinOmnichannelSystemIdentifier, Value: "cw-1"},
		},
		Telecom: []fhir_dto.ContactPoint{
			{System: fhir_dto.ContactPointSystemEmail, Value: "admin@test.com"},
			{System: fhir_dto.ContactPointSystemPhone, Value: "6281234567890"},
		},
	}

	t.Run("matching contact passes", func(t *testing.T) {
		mockPrac := new(mockPractitionerFhirClient)
		u := &usecase{practitionerFhir: mockPrac, log: zap.NewNop()}

		mockPrac.On("FindPractitionerByIdentifier", mock.Anything, constvars.FhirSupertokenSystemIdentifier, "st-123").
			Return([]fhir_dto.Practitioner{adminPrac}, nil)

		err := u.validateClinicAdminContact(context.Background(), "st-123", "admin@test.com", "6281234567890", "cw-1")
		assert.NoError(t, err)
		mockPrac.AssertExpectations(t)
	})

	t.Run("mismatched email rejected", func(t *testing.T) {
		mockPrac := new(mockPractitionerFhirClient)
		u := &usecase{practitionerFhir: mockPrac, log: zap.NewNop()}

		mockPrac.On("FindPractitionerByIdentifier", mock.Anything, constvars.FhirSupertokenSystemIdentifier, "st-123").
			Return([]fhir_dto.Practitioner{adminPrac}, nil)

		err := u.validateClinicAdminContact(context.Background(), "st-123", "other@test.com", "", "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "WEBHOOK_SYNC_CONTACT_MISMATCH")
	})

	t.Run("practitioner not found rejected", func(t *testing.T) {
		mockPrac := new(mockPractitionerFhirClient)
		u := &usecase{practitionerFhir: mockPrac, log: zap.NewNop()}

		mockPrac.On("FindPractitionerByIdentifier", mock.Anything, constvars.FhirSupertokenSystemIdentifier, "st-123").
			Return([]fhir_dto.Practitioner{}, nil)

		err := u.validateClinicAdminContact(context.Background(), "st-123", "admin@test.com", "", "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "WEBHOOK_SYNC_USER_NOT_FOUND")
	})
}
