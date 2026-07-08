package users

import (
	"context"
	"testing"

	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/fhir_dto"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// ---------------------------------------------------------------------------
// Mock types
// ---------------------------------------------------------------------------

type MockPractitionerFhirClient struct {
	mock.Mock
}

func (m *MockPractitionerFhirClient) CreatePractitioner(ctx context.Context, req *fhir_dto.Practitioner) (*fhir_dto.Practitioner, error) {
	args := m.Called(ctx, req)
	var out *fhir_dto.Practitioner
	if v := args.Get(0); v != nil {
		out = v.(*fhir_dto.Practitioner)
	}
	return out, args.Error(1)
}

func (m *MockPractitionerFhirClient) UpdatePractitioner(ctx context.Context, req *fhir_dto.Practitioner) (*fhir_dto.Practitioner, error) {
	args := m.Called(ctx, req)
	var out *fhir_dto.Practitioner
	if v := args.Get(0); v != nil {
		out = v.(*fhir_dto.Practitioner)
	}
	return out, args.Error(1)
}

func (m *MockPractitionerFhirClient) PatchPractitioner(ctx context.Context, req *fhir_dto.Practitioner) (*fhir_dto.Practitioner, error) {
	args := m.Called(ctx, req)
	var out *fhir_dto.Practitioner
	if v := args.Get(0); v != nil {
		out = v.(*fhir_dto.Practitioner)
	}
	return out, args.Error(1)
}

func (m *MockPractitionerFhirClient) FindPractitionerByID(ctx context.Context, id string) (*fhir_dto.Practitioner, error) {
	args := m.Called(ctx, id)
	var out *fhir_dto.Practitioner
	if v := args.Get(0); v != nil {
		out = v.(*fhir_dto.Practitioner)
	}
	return out, args.Error(1)
}

func (m *MockPractitionerFhirClient) FindPractitionerByIdentifier(ctx context.Context, system, value string) ([]fhir_dto.Practitioner, error) {
	args := m.Called(ctx, system, value)
	var out []fhir_dto.Practitioner
	if v := args.Get(0); v != nil {
		out = v.([]fhir_dto.Practitioner)
	}
	return out, args.Error(1)
}

func (m *MockPractitionerFhirClient) FindPractitionerByEmail(ctx context.Context, email string) ([]fhir_dto.Practitioner, error) {
	args := m.Called(ctx, email)
	var out []fhir_dto.Practitioner
	if v := args.Get(0); v != nil {
		out = v.([]fhir_dto.Practitioner)
	}
	return out, args.Error(1)
}

func (m *MockPractitionerFhirClient) FindPractitionerByPhone(ctx context.Context, phone string) ([]fhir_dto.Practitioner, error) {
	args := m.Called(ctx, phone)
	var out []fhir_dto.Practitioner
	if v := args.Get(0); v != nil {
		out = v.([]fhir_dto.Practitioner)
	}
	return out, args.Error(1)
}

type MockPatientFhirClient struct {
	mock.Mock
}

func (m *MockPatientFhirClient) CreatePatient(ctx context.Context, req *fhir_dto.Patient) (*fhir_dto.Patient, error) {
	args := m.Called(ctx, req)
	var out *fhir_dto.Patient
	if v := args.Get(0); v != nil {
		out = v.(*fhir_dto.Patient)
	}
	return out, args.Error(1)
}

func (m *MockPatientFhirClient) UpdatePatient(ctx context.Context, req *fhir_dto.Patient) (*fhir_dto.Patient, error) {
	args := m.Called(ctx, req)
	var out *fhir_dto.Patient
	if v := args.Get(0); v != nil {
		out = v.(*fhir_dto.Patient)
	}
	return out, args.Error(1)
}

func (m *MockPatientFhirClient) PatchPatient(ctx context.Context, req *fhir_dto.Patient) (*fhir_dto.Patient, error) {
	args := m.Called(ctx, req)
	var out *fhir_dto.Patient
	if v := args.Get(0); v != nil {
		out = v.(*fhir_dto.Patient)
	}
	return out, args.Error(1)
}

func (m *MockPatientFhirClient) FindPatientByID(ctx context.Context, id string) (*fhir_dto.Patient, error) {
	args := m.Called(ctx, id)
	var out *fhir_dto.Patient
	if v := args.Get(0); v != nil {
		out = v.(*fhir_dto.Patient)
	}
	return out, args.Error(1)
}

func (m *MockPatientFhirClient) FindPatientByIdentifier(ctx context.Context, identifier string) ([]fhir_dto.Patient, error) {
	args := m.Called(ctx, identifier)
	var out []fhir_dto.Patient
	if v := args.Get(0); v != nil {
		out = v.([]fhir_dto.Patient)
	}
	return out, args.Error(1)
}

func (m *MockPatientFhirClient) FindPatientByEmail(ctx context.Context, email string) ([]fhir_dto.Patient, error) {
	args := m.Called(ctx, email)
	var out []fhir_dto.Patient
	if v := args.Get(0); v != nil {
		out = v.([]fhir_dto.Patient)
	}
	return out, args.Error(1)
}

func (m *MockPatientFhirClient) FindPatientByPhone(ctx context.Context, phone string) ([]fhir_dto.Patient, error) {
	args := m.Called(ctx, phone)
	var out []fhir_dto.Patient
	if v := args.Get(0); v != nil {
		out = v.([]fhir_dto.Patient)
	}
	return out, args.Error(1)
}

type MockPersonFhirClient struct {
	mock.Mock
}

func (m *MockPersonFhirClient) FindPersonByEmail(ctx context.Context, email string) ([]fhir_dto.Person, error) {
	args := m.Called(ctx, email)
	var out []fhir_dto.Person
	if v := args.Get(0); v != nil {
		out = v.([]fhir_dto.Person)
	}
	return out, args.Error(1)
}

func (m *MockPersonFhirClient) FindPersonByPhone(ctx context.Context, phone string) ([]fhir_dto.Person, error) {
	args := m.Called(ctx, phone)
	var out []fhir_dto.Person
	if v := args.Get(0); v != nil {
		out = v.([]fhir_dto.Person)
	}
	return out, args.Error(1)
}

func (m *MockPersonFhirClient) Create(ctx context.Context, person *fhir_dto.Person) (*fhir_dto.Person, error) {
	args := m.Called(ctx, person)
	var out *fhir_dto.Person
	if v := args.Get(0); v != nil {
		out = v.(*fhir_dto.Person)
	}
	return out, args.Error(1)
}

func (m *MockPersonFhirClient) Search(ctx context.Context, params contracts.PersonSearchInput) ([]fhir_dto.Person, error) {
	args := m.Called(ctx, params)
	var out []fhir_dto.Person
	if v := args.Get(0); v != nil {
		out = v.([]fhir_dto.Person)
	}
	return out, args.Error(1)
}

func (m *MockPersonFhirClient) Update(ctx context.Context, person *fhir_dto.Person) (*fhir_dto.Person, error) {
	args := m.Called(ctx, person)
	var out *fhir_dto.Person
	if v := args.Get(0); v != nil {
		out = v.(*fhir_dto.Person)
	}
	return out, args.Error(1)
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

func newTestUsecase(pracClient contracts.PractitionerFhirClient, patClient contracts.PatientFhirClient, personClient contracts.PersonFhirClient) *userUsecase {
	return &userUsecase{
		PractitionerFhirClient: pracClient,
		PatientFhirClient:      patClient,
		PersonFhirClient:       personClient,
		Log:                    zap.NewNop(),
	}
}

func ctx() context.Context {
	return context.Background()
}

// ---------------------------------------------------------------------------
// scanIdentifiers
// ---------------------------------------------------------------------------

func TestScanIdentifiers_FindsSupertokenExact(t *testing.T) {
	ids := []fhir_dto.Identifier{
		{System: constvars.FhirSupertokenSystemIdentifier, Value: "st-123"},
		{System: constvars.KonsulinOmnichannelSystemIdentifier, Value: "42"},
	}

	result := scanIdentifiers(ids, "st-123", "42")

	assert.True(t, result.foundSupertoken)
	assert.Equal(t, 0, result.foundSupertokenIdx)
	assert.True(t, result.supertokenExactMatch)
	assert.True(t, result.foundChatwoot)
	assert.Equal(t, 1, result.foundChatwootIdx)
	assert.True(t, result.chatwootExactMatch)
}

func TestScanIdentifiers_NoMatch(t *testing.T) {
	ids := []fhir_dto.Identifier{
		{System: constvars.FhirSupertokenSystemIdentifier, Value: "st-123"},
	}

	result := scanIdentifiers(ids, "st-999", "42")

	assert.True(t, result.foundSupertoken)
	assert.Equal(t, 0, result.foundSupertokenIdx)
	assert.False(t, result.supertokenExactMatch)
	assert.False(t, result.foundChatwoot)
	assert.Equal(t, -1, result.foundChatwootIdx)
	assert.False(t, result.chatwootExactMatch)
}

func TestScanIdentifiers_EmptySlice(t *testing.T) {
	result := scanIdentifiers([]fhir_dto.Identifier{}, "st-123", "42")

	assert.False(t, result.foundSupertoken)
	assert.Equal(t, -1, result.foundSupertokenIdx)
	assert.False(t, result.supertokenExactMatch)
	assert.False(t, result.foundChatwoot)
	assert.Equal(t, -1, result.foundChatwootIdx)
}

func TestScanIdentifiers_MultipleIdentifiers(t *testing.T) {
	ids := []fhir_dto.Identifier{
		{System: constvars.FhirSupertokenSystemIdentifier, Value: "st-old"},
		{System: "http://other.system", Value: "other-val"},
		{System: constvars.FhirSupertokenSystemIdentifier, Value: "st-123"},
	}

	result := scanIdentifiers(ids, "st-123", "")

	assert.True(t, result.foundSupertoken)
	assert.Equal(t, 2, result.foundSupertokenIdx)
	assert.True(t, result.supertokenExactMatch)
	assert.False(t, result.foundChatwoot)
	assert.Equal(t, -1, result.foundChatwootIdx)
}

// ---------------------------------------------------------------------------
// ensurePractitionerIdentifiers
// ---------------------------------------------------------------------------

func TestEnsurePractitionerIdentifiers_ExactMatch_NoUpdate(t *testing.T) {
	// A practitioner with correct supertoken already set → no Update call
	mockPrac := new(MockPractitionerFhirClient)
	uc := newTestUsecase(mockPrac, nil, nil)

	prac := &fhir_dto.Practitioner{
		Identifier: []fhir_dto.Identifier{
			{System: constvars.FhirSupertokenSystemIdentifier, Value: "st-123"},
		},
	}

	// No mock expectations → UpdatePractitioner should NOT be called
	result, err := uc.ensurePractitionerIdentifiers(ctx(), prac, "", "", "st-123")

	require.NoError(t, err)
	require.NotNil(t, result)
	// Must return same pointer (no allocation)
	assert.Equal(t, prac, result)
	assert.Len(t, result.Identifier, 1)
	assert.Equal(t, "st-123", result.Identifier[0].Value)
	mockPrac.AssertExpectations(t)
}

func TestEnsurePractitionerIdentifiers_SupertokenMismatch_Updates(t *testing.T) {
	// Practitioner has wrong supertoken → must update
	mockPrac := new(MockPractitionerFhirClient)
	uc := newTestUsecase(mockPrac, nil, nil)

	prac := &fhir_dto.Practitioner{
		Identifier: []fhir_dto.Identifier{
			{System: constvars.FhirSupertokenSystemIdentifier, Value: "st-old"},
		},
	}

	expected := &fhir_dto.Practitioner{
		Identifier: []fhir_dto.Identifier{
			{System: constvars.FhirSupertokenSystemIdentifier, Value: "st-123"},
		},
	}

	mockPrac.On("UpdatePractitioner", ctx(), mock.MatchedBy(func(p *fhir_dto.Practitioner) bool {
		return len(p.Identifier) == 1 && p.Identifier[0].Value == "st-123"
	})).Return(expected, nil)

	result, err := uc.ensurePractitionerIdentifiers(ctx(), prac, "", "", "st-123")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "st-123", result.Identifier[0].Value)
	mockPrac.AssertExpectations(t)
}

func TestEnsurePractitionerIdentifiers_SupertokenMissing_Appends(t *testing.T) {
	// Practitioner has no supertoken identifier → append
	mockPrac := new(MockPractitionerFhirClient)
	uc := newTestUsecase(mockPrac, nil, nil)

	prac := &fhir_dto.Practitioner{
		Identifier: []fhir_dto.Identifier{
			{System: "http://other.system", Value: "other"},
		},
	}

	expected := &fhir_dto.Practitioner{
		Identifier: []fhir_dto.Identifier{
			{System: "http://other.system", Value: "other"},
			{System: constvars.FhirSupertokenSystemIdentifier, Value: "st-123"},
		},
	}

	mockPrac.On("UpdatePractitioner", ctx(), mock.MatchedBy(func(p *fhir_dto.Practitioner) bool {
		return len(p.Identifier) == 2
	})).Return(expected, nil)

	result, err := uc.ensurePractitionerIdentifiers(ctx(), prac, "", "", "st-123")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Len(t, result.Identifier, 2)
	assert.Equal(t, "st-123", result.Identifier[1].Value)
	mockPrac.AssertExpectations(t)
}

func TestEnsurePractitionerIdentifiers_EmptySupertoken_NoUpdate(t *testing.T) {
	// superTokenUserID is empty → update not needed even if identifiers exist
	mockPrac := new(MockPractitionerFhirClient)
	uc := newTestUsecase(mockPrac, nil, nil)

	prac := &fhir_dto.Practitioner{
		Identifier: []fhir_dto.Identifier{
			{System: constvars.FhirSupertokenSystemIdentifier, Value: "st-123"},
		},
	}

	result, err := uc.ensurePractitionerIdentifiers(ctx(), prac, "", "", "")

	require.NoError(t, err)
	assert.Equal(t, prac, result)
	mockPrac.AssertExpectations(t)
}
