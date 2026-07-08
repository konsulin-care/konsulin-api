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

// ---------------------------------------------------------------------------
// lookupPractitioner
// ---------------------------------------------------------------------------

func TestLookupPractitioner_ByEmail(t *testing.T) {
	mockPrac := new(MockPractitionerFhirClient)
	uc := newTestUsecase(mockPrac, nil, nil)
	expected := []fhir_dto.Practitioner{{ResourceType: "Practitioner"}}

	mockPrac.On("FindPractitionerByEmail", ctx(), "doc@test.com").Return(expected, nil)

	result, err := uc.lookupPractitioner(ctx(), "doc@test.com", "", "")

	require.NoError(t, err)
	assert.Equal(t, expected, result)
	mockPrac.AssertExpectations(t)
}

func TestLookupPractitioner_ByPhone(t *testing.T) {
	mockPrac := new(MockPractitionerFhirClient)
	uc := newTestUsecase(mockPrac, nil, nil)
	expected := []fhir_dto.Practitioner{{ResourceType: "Practitioner"}}

	mockPrac.On("FindPractitionerByPhone", ctx(), "6281234567890").Return(expected, nil)

	result, err := uc.lookupPractitioner(ctx(), "", "6281234567890", "")

	require.NoError(t, err)
	assert.Equal(t, expected, result)
	mockPrac.AssertExpectations(t)
}

func TestLookupPractitioner_ByIdentifier(t *testing.T) {
	mockPrac := new(MockPractitionerFhirClient)
	uc := newTestUsecase(mockPrac, nil, nil)
	expected := []fhir_dto.Practitioner{{ResourceType: "Practitioner"}}

	mockPrac.On("FindPractitionerByIdentifier", ctx(),
		constvars.FhirSupertokenSystemIdentifier, "st-123").Return(expected, nil)

	result, err := uc.lookupPractitioner(ctx(), "", "", "st-123")

	require.NoError(t, err)
	assert.Equal(t, expected, result)
	mockPrac.AssertExpectations(t)
}

func TestLookupPractitioner_AllEmpty(t *testing.T) {
	mockPrac := new(MockPractitionerFhirClient)
	uc := newTestUsecase(mockPrac, nil, nil)

	result, err := uc.lookupPractitioner(ctx(), "", "", "")

	require.NoError(t, err)
	assert.Nil(t, result)
}

// ---------------------------------------------------------------------------
// ensurePatientIdentifiers
// ---------------------------------------------------------------------------

func TestEnsurePatientIdentifiers_ExactMatch_NoUpdate(t *testing.T) {
	mockPat := new(MockPatientFhirClient)
	uc := newTestUsecase(nil, mockPat, nil)

	patient := &fhir_dto.Patient{
		Identifier: []fhir_dto.Identifier{
			{System: constvars.FhirSupertokenSystemIdentifier, Value: "st-123"},
		},
	}

	result, err := uc.ensurePatientIdentifiers(ctx(), patient, "", "", "st-123")

	require.NoError(t, err)
	assert.Equal(t, patient, result)
	mockPat.AssertExpectations(t)
}

func TestEnsurePatientIdentifiers_Mismatch_Updates(t *testing.T) {
	mockPat := new(MockPatientFhirClient)
	uc := newTestUsecase(nil, mockPat, nil)

	patient := &fhir_dto.Patient{
		Identifier: []fhir_dto.Identifier{
			{System: constvars.FhirSupertokenSystemIdentifier, Value: "st-old"},
		},
	}
	expected := &fhir_dto.Patient{
		Identifier: []fhir_dto.Identifier{
			{System: constvars.FhirSupertokenSystemIdentifier, Value: "st-123"},
		},
	}

	mockPat.On("UpdatePatient", ctx(), mock.Anything).Return(expected, nil)

	result, err := uc.ensurePatientIdentifiers(ctx(), patient, "", "", "st-123")

	require.NoError(t, err)
	assert.Equal(t, "st-123", result.Identifier[0].Value)
	mockPat.AssertExpectations(t)
}

// ---------------------------------------------------------------------------
// ensurePersonIdentifiers
// ---------------------------------------------------------------------------

func TestEnsurePersonIdentifiers_ExactMatch_NoUpdate(t *testing.T) {
	mockPerson := new(MockPersonFhirClient)
	uc := newTestUsecase(nil, nil, mockPerson)

	person := &fhir_dto.Person{
		Identifier: []fhir_dto.Identifier{
			{System: constvars.FhirSupertokenSystemIdentifier, Value: "st-123"},
		},
	}

	result, err := uc.ensurePersonIdentifiers(ctx(), person, "", "", "st-123")

	require.NoError(t, err)
	assert.Equal(t, person, result)
	mockPerson.AssertExpectations(t)
}

func TestEnsurePersonIdentifiers_Mismatch_Updates(t *testing.T) {
	mockPerson := new(MockPersonFhirClient)
	uc := newTestUsecase(nil, nil, mockPerson)

	person := &fhir_dto.Person{
		Identifier: []fhir_dto.Identifier{
			{System: constvars.FhirSupertokenSystemIdentifier, Value: "st-old"},
		},
	}
	expected := &fhir_dto.Person{
		Identifier: []fhir_dto.Identifier{
			{System: constvars.FhirSupertokenSystemIdentifier, Value: "st-123"},
		},
	}

	mockPerson.On("Update", ctx(), mock.Anything).Return(expected, nil)

	result, err := uc.ensurePersonIdentifiers(ctx(), person, "", "", "st-123")

	require.NoError(t, err)
	assert.Equal(t, "st-123", result.Identifier[0].Value)
	mockPerson.AssertExpectations(t)
}

// ---------------------------------------------------------------------------
// createNewPractitioner
// ---------------------------------------------------------------------------

func TestCreateNewPractitioner_MissingSupertoken_Error(t *testing.T) {
	mockPrac := new(MockPractitionerFhirClient)
	uc := newTestUsecase(mockPrac, nil, nil)

	_, err := uc.createNewPractitioner(ctx(), "doc@test.com", "", "")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "superTokenUserID")
}

func TestCreateNewPractitioner_Success(t *testing.T) {
	// Pass empty email and phone to avoid triggering the Chatwoot webhook call
	// (which requires JWTTokenManager and HTTP server setup).
	mockPrac := new(MockPractitionerFhirClient)
	uc := newTestUsecase(mockPrac, nil, nil)

	newPrac := &fhir_dto.Practitioner{
		ID: "prac-1", ResourceType: "Practitioner", Active: true,
	}

	mockPrac.On("CreatePractitioner", ctx(), mock.MatchedBy(func(p *fhir_dto.Practitioner) bool {
		return p.ResourceType == "Practitioner" && p.Active &&
			len(p.Identifier) == 1 &&
			p.Identifier[0].System == constvars.FhirSupertokenSystemIdentifier &&
			p.Identifier[0].Value == "st-123"
	})).Return(newPrac, nil)

	result, err := uc.createNewPractitioner(ctx(), "", "", "st-123")

	require.NoError(t, err)
	assert.Equal(t, "prac-1", result.ID)
	mockPrac.AssertExpectations(t)
}

func TestCreateNewPractitioner_FhirClientError(t *testing.T) {
	mockPrac := new(MockPractitionerFhirClient)
	uc := newTestUsecase(mockPrac, nil, nil)

	mockPrac.On("CreatePractitioner", ctx(), mock.Anything).Return(nil, assert.AnError)

	_, err := uc.createNewPractitioner(ctx(), "", "", "st-123")

	require.Error(t, err)
	mockPrac.AssertExpectations(t)
}

// ---------------------------------------------------------------------------
// createPractitionerIfNotExists (orchestrator)
// ---------------------------------------------------------------------------

func TestCreatePractitionerIfNotExists_Found_EnsuresIdentifiers(t *testing.T) {
	// Use identifier-based lookup to avoid triggering Chatwoot (requires JWTTokenManager).
	// Both email and phone are empty so the Chatwoot guard returns early.
	mockPrac := new(MockPractitionerFhirClient)
	uc := newTestUsecase(mockPrac, nil, nil)

	existing := []fhir_dto.Practitioner{
		{Identifier: []fhir_dto.Identifier{
			{System: constvars.FhirSupertokenSystemIdentifier, Value: "st-old"},
		}},
	}
	updated := &fhir_dto.Practitioner{
		Identifier: []fhir_dto.Identifier{
			{System: constvars.FhirSupertokenSystemIdentifier, Value: "st-123"},
		},
	}

	mockPrac.On("FindPractitionerByIdentifier", ctx(),
		constvars.FhirSupertokenSystemIdentifier, "st-123").Return(existing, nil)
	mockPrac.On("UpdatePractitioner", ctx(), mock.Anything).Return(updated, nil)

	result, err := uc.createPractitionerIfNotExists(ctx(), "", "", "st-123")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "st-123", result.Identifier[0].Value)
	mockPrac.AssertExpectations(t)
}

func TestCreatePractitionerIfNotExists_NotFound_CreatesNew(t *testing.T) {
	mockPrac := new(MockPractitionerFhirClient)
	uc := newTestUsecase(mockPrac, nil, nil)

	created := &fhir_dto.Practitioner{ID: "prac-new", ResourceType: "Practitioner", Active: true}

	mockPrac.On("FindPractitionerByIdentifier", ctx(),
		constvars.FhirSupertokenSystemIdentifier, "st-123").Return([]fhir_dto.Practitioner{}, nil)
	mockPrac.On("CreatePractitioner", ctx(), mock.Anything).Return(created, nil)

	result, err := uc.createPractitionerIfNotExists(ctx(), "", "", "st-123")

	require.NoError(t, err)
	assert.Equal(t, "prac-new", result.ID)
	mockPrac.AssertExpectations(t)
}

func TestCreatePractitionerIfNotExists_LookupError_Propagates(t *testing.T) {
	mockPrac := new(MockPractitionerFhirClient)
	uc := newTestUsecase(mockPrac, nil, nil)

	mockPrac.On("FindPractitionerByIdentifier", ctx(),
		constvars.FhirSupertokenSystemIdentifier, "st-123").Return(nil, assert.AnError)

	_, err := uc.createPractitionerIfNotExists(ctx(), "", "", "st-123")

	require.Error(t, err)
	mockPrac.AssertExpectations(t)
}

// ---------------------------------------------------------------------------
// createPatientIfNotExists (orchestrator)
// ---------------------------------------------------------------------------

func TestCreatePatientIfNotExists_Found_EnsuresIdentifiers(t *testing.T) {
	mockPat := new(MockPatientFhirClient)
	uc := newTestUsecase(nil, mockPat, nil)

	identifierQuery := constvars.FhirSupertokenSystemIdentifier + "|" + "st-123"
	existing := []fhir_dto.Patient{
		{Identifier: []fhir_dto.Identifier{
			{System: constvars.FhirSupertokenSystemIdentifier, Value: "st-old"},
		}},
	}
	updated := &fhir_dto.Patient{
		Identifier: []fhir_dto.Identifier{
			{System: constvars.FhirSupertokenSystemIdentifier, Value: "st-123"},
		},
	}

	mockPat.On("FindPatientByIdentifier", ctx(), identifierQuery).Return(existing, nil)
	mockPat.On("UpdatePatient", ctx(), mock.Anything).Return(updated, nil)

	result, err := uc.createPatientIfNotExists(ctx(), "", "", "st-123")

	require.NoError(t, err)
	assert.Equal(t, "st-123", result.Identifier[0].Value)
	mockPat.AssertExpectations(t)
}

func TestCreatePatientIfNotExists_NotFound_CreatesNew(t *testing.T) {
	mockPat := new(MockPatientFhirClient)
	uc := newTestUsecase(nil, mockPat, nil)

	identifierQuery := constvars.FhirSupertokenSystemIdentifier + "|" + "st-123"
	created := &fhir_dto.Patient{ID: "pat-new", ResourceType: "Patient", Active: true}

	mockPat.On("FindPatientByIdentifier", ctx(), identifierQuery).Return([]fhir_dto.Patient{}, nil)
	mockPat.On("CreatePatient", ctx(), mock.Anything).Return(created, nil)

	result, err := uc.createPatientIfNotExists(ctx(), "", "", "st-123")

	require.NoError(t, err)
	assert.Equal(t, "pat-new", result.ID)
	mockPat.AssertExpectations(t)
}

// ---------------------------------------------------------------------------
// createPersonIfNotExists (orchestrator)
// ---------------------------------------------------------------------------

func TestCreatePersonIfNotExists_Found_EnsuresIdentifiers(t *testing.T) {
	mockPerson := new(MockPersonFhirClient)
	uc := newTestUsecase(nil, nil, mockPerson)

	searchInput := contracts.PersonSearchInput{
		Identifier: constvars.FhirSupertokenSystemIdentifier + "|" + "st-123",
	}
	existing := []fhir_dto.Person{
		{Identifier: []fhir_dto.Identifier{
			{System: constvars.FhirSupertokenSystemIdentifier, Value: "st-old"},
		}},
	}
	updated := &fhir_dto.Person{
		Identifier: []fhir_dto.Identifier{
			{System: constvars.FhirSupertokenSystemIdentifier, Value: "st-123"},
		},
	}

	mockPerson.On("Search", ctx(), searchInput).Return(existing, nil)
	mockPerson.On("Update", ctx(), mock.Anything).Return(updated, nil)

	result, err := uc.createPersonIfNotExists(ctx(), "", "", "st-123")

	require.NoError(t, err)
	assert.Equal(t, "st-123", result.Identifier[0].Value)
	mockPerson.AssertExpectations(t)
}

func TestCreatePersonIfNotExists_NotFound_CreatesNew(t *testing.T) {
	mockPerson := new(MockPersonFhirClient)
	uc := newTestUsecase(nil, nil, mockPerson)

	searchInput := contracts.PersonSearchInput{
		Identifier: constvars.FhirSupertokenSystemIdentifier + "|" + "st-123",
	}
	created := &fhir_dto.Person{ID: "per-new", ResourceType: "Person", Active: true}

	mockPerson.On("Search", ctx(), searchInput).Return([]fhir_dto.Person{}, nil)
	mockPerson.On("Create", ctx(), mock.Anything).Return(created, nil)

	result, err := uc.createPersonIfNotExists(ctx(), "", "", "st-123")

	require.NoError(t, err)
	assert.Equal(t, "per-new", result.ID)
	mockPerson.AssertExpectations(t)
}
