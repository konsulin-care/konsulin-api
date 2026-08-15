package webhook

import (
	"context"
	"testing"
	"time"

	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/services/shared/webhookqueue"
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

// TestValidatePractitionerContact verifies practitioner contact validation
// resolves the Practitioner by supertoken identifier (clinic admins share this
// path; the Person identity was dropped).
func TestValidatePractitionerContact(t *testing.T) {
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

		err := u.validatePractitionerContact(context.Background(), "st-123", "admin@test.com", "6281234567890", "cw-1")
		assert.NoError(t, err)
		mockPrac.AssertExpectations(t)
	})

	t.Run("mismatched email rejected", func(t *testing.T) {
		mockPrac := new(mockPractitionerFhirClient)
		u := &usecase{practitionerFhir: mockPrac, log: zap.NewNop()}

		mockPrac.On("FindPractitionerByIdentifier", mock.Anything, constvars.FhirSupertokenSystemIdentifier, "st-123").
			Return([]fhir_dto.Practitioner{adminPrac}, nil)

		err := u.validatePractitionerContact(context.Background(), "st-123", "other@test.com", "", "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "WEBHOOK_SYNC_CONTACT_MISMATCH")
	})

	t.Run("practitioner not found rejected", func(t *testing.T) {
		mockPrac := new(mockPractitionerFhirClient)
		u := &usecase{practitionerFhir: mockPrac, log: zap.NewNop()}

		mockPrac.On("FindPractitionerByIdentifier", mock.Anything, constvars.FhirSupertokenSystemIdentifier, "st-123").
			Return([]fhir_dto.Practitioner{}, nil)

		err := u.validatePractitionerContact(context.Background(), "st-123", "admin@test.com", "", "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "WEBHOOK_SYNC_USER_NOT_FOUND")
	})
}

// TestValidateContactByRoleClinicAdmin pins that the role dispatcher validates
// clinic admins against their Practitioner identity (same path as
// practitioners; the Person identity was dropped).
func TestValidateContactByRoleClinicAdmin(t *testing.T) {
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

		err := u.validateContactByRole(context.Background(), constvars.KonsulinRoleClinicAdmin, "st-123", "admin@test.com", "6281234567890", "cw-1")
		assert.NoError(t, err)
		mockPrac.AssertExpectations(t)
	})

	t.Run("mismatched email rejected", func(t *testing.T) {
		mockPrac := new(mockPractitionerFhirClient)
		u := &usecase{practitionerFhir: mockPrac, log: zap.NewNop()}

		mockPrac.On("FindPractitionerByIdentifier", mock.Anything, constvars.FhirSupertokenSystemIdentifier, "st-123").
			Return([]fhir_dto.Practitioner{adminPrac}, nil)

		err := u.validateContactByRole(context.Background(), constvars.KonsulinRoleClinicAdmin, "st-123", "other@test.com", "", "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "WEBHOOK_SYNC_CONTACT_MISMATCH")
	})

	t.Run("practitioner not found rejected", func(t *testing.T) {
		mockPrac := new(mockPractitionerFhirClient)
		u := &usecase{practitionerFhir: mockPrac, log: zap.NewNop()}

		mockPrac.On("FindPractitionerByIdentifier", mock.Anything, constvars.FhirSupertokenSystemIdentifier, "st-123").
			Return([]fhir_dto.Practitioner{}, nil)

		err := u.validateContactByRole(context.Background(), constvars.KonsulinRoleClinicAdmin, "st-123", "admin@test.com", "", "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "WEBHOOK_SYNC_USER_NOT_FOUND")
	})
}

// TestNewUsecase verifies the Options struct wires every dependency into the
// usecase: config-derived sync set, failure policy, and HTTP timeout.
func TestNewUsecase(t *testing.T) {
	cfg := &config.InternalConfig{Webhook: config.AppWebhook{
		SynchronousServiceNames:         []string{"analyze", " Report "},
		SynchronousServiceFailurePolicy: "enqueue_request",
		HTTPTimeoutInSeconds:            3,
	}}

	u := NewUsecase(Options{
		Log:      zap.NewNop(),
		Config:   cfg,
		Queue:    &webhookqueue.Service{},
		Enforcer: nil,
	})

	uc, ok := u.(*usecase)
	assert.True(t, ok)
	assert.Same(t, cfg, uc.cfg)
	assert.NotNil(t, uc.log)
	assert.NotNil(t, uc.queue)
	assert.Contains(t, uc.syncServiceSet, "analyze")
	assert.Contains(t, uc.syncServiceSet, "report")
	assert.Equal(t, SyncFailurePolicyEnqueueRequest, uc.failurePolicy)
	assert.Equal(t, 3*time.Second, uc.httpClient.Timeout)
}

func TestNewUsecaseDefaultsFailurePolicyToReturnError(t *testing.T) {
	cfg := &config.InternalConfig{Webhook: config.AppWebhook{
		SynchronousServiceNames:         nil,
		SynchronousServiceFailurePolicy: "",
	}}

	u := NewUsecase(Options{Log: zap.NewNop(), Config: cfg})

	uc := u.(*usecase)
	assert.Empty(t, uc.syncServiceSet)
	assert.Equal(t, SyncFailurePolicyReturnError, uc.failurePolicy)
	assert.Equal(t, 10*time.Second, uc.httpClient.Timeout)
}
