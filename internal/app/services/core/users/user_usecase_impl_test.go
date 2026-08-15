package users

import (
	"context"
	"testing"

	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/fhir_dto"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// ---------------------------------------------------------------------------
// Generic mock helpers
// ---------------------------------------------------------------------------

// mockResult collapses (*T, error) mock methods to a one-liner.
func mockResult[T any](args mock.Arguments) (*T, error) {
	var out *T
	if v := args.Get(0); v != nil {
		out = v.(*T)
	}
	return out, args.Error(1)
}

// mockSliceResult collapses ([]T, error) mock methods to a one-liner.
func mockSliceResult[T any](args mock.Arguments) ([]T, error) {
	var out []T
	if v := args.Get(0); v != nil {
		out = v.([]T)
	}
	return out, args.Error(1)
}

// ---------------------------------------------------------------------------
// Mock types
// ---------------------------------------------------------------------------

type MockPractitionerFhirClient struct {
	mock.Mock
}

func (m *MockPractitionerFhirClient) CreatePractitioner(ctx context.Context, req *fhir_dto.Practitioner) (*fhir_dto.Practitioner, error) {
	return mockResult[fhir_dto.Practitioner](m.Called(ctx, req))
}

func (m *MockPractitionerFhirClient) UpdatePractitioner(ctx context.Context, req *fhir_dto.Practitioner) (*fhir_dto.Practitioner, error) {
	return mockResult[fhir_dto.Practitioner](m.Called(ctx, req))
}

func (m *MockPractitionerFhirClient) PatchPractitioner(ctx context.Context, req *fhir_dto.Practitioner) (*fhir_dto.Practitioner, error) {
	return mockResult[fhir_dto.Practitioner](m.Called(ctx, req))
}

func (m *MockPractitionerFhirClient) FindPractitionerByID(ctx context.Context, id string) (*fhir_dto.Practitioner, error) {
	return mockResult[fhir_dto.Practitioner](m.Called(ctx, id))
}

func (m *MockPractitionerFhirClient) FindPractitionerByIdentifier(ctx context.Context, system, value string) ([]fhir_dto.Practitioner, error) {
	return mockSliceResult[fhir_dto.Practitioner](m.Called(ctx, system, value))
}

func (m *MockPractitionerFhirClient) FindPractitionerByEmail(ctx context.Context, email string) ([]fhir_dto.Practitioner, error) {
	return mockSliceResult[fhir_dto.Practitioner](m.Called(ctx, email))
}

func (m *MockPractitionerFhirClient) FindPractitionerByPhone(ctx context.Context, phone string) ([]fhir_dto.Practitioner, error) {
	return mockSliceResult[fhir_dto.Practitioner](m.Called(ctx, phone))
}

type MockPatientFhirClient struct {
	mock.Mock
}

func (m *MockPatientFhirClient) CreatePatient(ctx context.Context, req *fhir_dto.Patient) (*fhir_dto.Patient, error) {
	return mockResult[fhir_dto.Patient](m.Called(ctx, req))
}

func (m *MockPatientFhirClient) UpdatePatient(ctx context.Context, req *fhir_dto.Patient) (*fhir_dto.Patient, error) {
	return mockResult[fhir_dto.Patient](m.Called(ctx, req))
}

func (m *MockPatientFhirClient) PatchPatient(ctx context.Context, req *fhir_dto.Patient) (*fhir_dto.Patient, error) {
	return mockResult[fhir_dto.Patient](m.Called(ctx, req))
}

func (m *MockPatientFhirClient) FindPatientByID(ctx context.Context, id string) (*fhir_dto.Patient, error) {
	return mockResult[fhir_dto.Patient](m.Called(ctx, id))
}

func (m *MockPatientFhirClient) FindPatientByIdentifier(ctx context.Context, identifier string) ([]fhir_dto.Patient, error) {
	return mockSliceResult[fhir_dto.Patient](m.Called(ctx, identifier))
}

func (m *MockPatientFhirClient) FindPatientByEmail(ctx context.Context, email string) ([]fhir_dto.Patient, error) {
	return mockSliceResult[fhir_dto.Patient](m.Called(ctx, email))
}

func (m *MockPatientFhirClient) FindPatientByPhone(ctx context.Context, phone string) ([]fhir_dto.Patient, error) {
	return mockSliceResult[fhir_dto.Patient](m.Called(ctx, phone))
}

type MockPractitionerRoleFhirClient struct {
	mock.Mock
}

func (m *MockPractitionerRoleFhirClient) DeletePractitionerRoleByID(ctx context.Context, practitionerRoleID string) error {
	return m.Called(ctx, practitionerRoleID).Error(0)
}

func (m *MockPractitionerRoleFhirClient) FindPractitionerRoleByOrganizationID(ctx context.Context, organizationID string) ([]fhir_dto.PractitionerRole, error) {
	return mockSliceResult[fhir_dto.PractitionerRole](m.Called(ctx, organizationID))
}

func (m *MockPractitionerRoleFhirClient) FindPractitionerRoleByCustomRequest(ctx context.Context, request *requests.FindAllCliniciansByClinicID) ([]fhir_dto.PractitionerRole, error) {
	return mockSliceResult[fhir_dto.PractitionerRole](m.Called(ctx, request))
}

func (m *MockPractitionerRoleFhirClient) FindPractitionerRoleByPractitionerID(ctx context.Context, practitionerID string) ([]fhir_dto.PractitionerRole, error) {
	return mockSliceResult[fhir_dto.PractitionerRole](m.Called(ctx, practitionerID))
}

func (m *MockPractitionerRoleFhirClient) FindPractitionerRoleByPractitionerIDAndOrganizationID(ctx context.Context, practitionerID, organizationID string) ([]fhir_dto.PractitionerRole, error) {
	return mockSliceResult[fhir_dto.PractitionerRole](m.Called(ctx, practitionerID, organizationID))
}

func (m *MockPractitionerRoleFhirClient) CreatePractitionerRoles(ctx context.Context, request interface{}) error {
	return m.Called(ctx, request).Error(0)
}

func (m *MockPractitionerRoleFhirClient) CreatePractitionerRole(ctx context.Context, request *fhir_dto.PractitionerRole) (*fhir_dto.PractitionerRole, error) {
	return mockResult[fhir_dto.PractitionerRole](m.Called(ctx, request))
}

func (m *MockPractitionerRoleFhirClient) UpdatePractitionerRole(ctx context.Context, request *fhir_dto.PractitionerRole) (*fhir_dto.PractitionerRole, error) {
	return mockResult[fhir_dto.PractitionerRole](m.Called(ctx, request))
}

func (m *MockPractitionerRoleFhirClient) FindPractitionerRoleByPractitionerIDAndName(ctx context.Context, request *requests.FindClinicianByClinicianID) ([]fhir_dto.PractitionerRole, error) {
	return mockSliceResult[fhir_dto.PractitionerRole](m.Called(ctx, request))
}

func (m *MockPractitionerRoleFhirClient) FindPractitionerRoleByID(ctx context.Context, practitionerRoleID string) (*fhir_dto.PractitionerRole, error) {
	return mockResult[fhir_dto.PractitionerRole](m.Called(ctx, practitionerRoleID))
}

func (m *MockPractitionerRoleFhirClient) Search(ctx context.Context, params contracts.PractitionerRoleSearchParams) ([]fhir_dto.PractitionerRole, error) {
	return mockSliceResult[fhir_dto.PractitionerRole](m.Called(ctx, params))
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

func newTestUsecase(pracClient contracts.PractitionerFhirClient, patClient contracts.PatientFhirClient, prClient contracts.PractitionerRoleFhirClient) *userUsecase {
	return &userUsecase{
		PractitionerFhirClient:     pracClient,
		PatientFhirClient:          patClient,
		PractitionerRoleFhirClient: prClient,
		Log:                        zap.NewNop(),
	}
}

func ctx() context.Context {
	return context.Background()
}

// stubWebhookForwarder returns a webhookForwardFn that short-circuits the
// Chatwoot omnichannel call so createNewPractitioner/createNewPatient tests do
// not need JWTTokenManager or an HTTP server.
func stubWebhookForwarder() func(ctx context.Context, service, method string, body []byte, contentType string) (int, []byte, error) {
	return func(context.Context, string, string, []byte, string) (int, []byte, error) {
		return 200, []byte(`[{"chatwoot_id":123,"email":"test@example.com"}]`), nil
	}
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
// lookupPatient / lookupPractitioner: uid-first priority
// ---------------------------------------------------------------------------

func TestLookupPatient_IdentifierMatch_SkipsEmail(t *testing.T) {
	// The supertoken uid identifier is the stable per-user key. When it
	// matches, the email search must never run: email is not reliably indexed
	// on Blaze, so an email-first lookup lets duplicate Patients accumulate
	// under one uid across repeated logins.
	mockPat := new(MockPatientFhirClient)
	uc := newTestUsecase(nil, mockPat, nil)
	expected := []fhir_dto.Patient{{ResourceType: "Patient"}}
	identifierQuery := constvars.FhirSupertokenSystemIdentifier + "|st-123"

	mockPat.On("FindPatientByIdentifier", ctx(), identifierQuery).Return(expected, nil)

	result, err := uc.lookupPatient(ctx(), "pat@test.com", "", "st-123")

	require.NoError(t, err)
	assert.Equal(t, expected, result)
	mockPat.AssertNotCalled(t, "FindPatientByEmail", mock.Anything)
	mockPat.AssertExpectations(t)
}

func TestLookupPractitioner_IdentifierMatch_SkipsEmail(t *testing.T) {
	mockPrac := new(MockPractitionerFhirClient)
	uc := newTestUsecase(mockPrac, nil, nil)
	expected := []fhir_dto.Practitioner{{ResourceType: "Practitioner"}}

	mockPrac.On("FindPractitionerByIdentifier", ctx(),
		constvars.FhirSupertokenSystemIdentifier, "st-123").Return(expected, nil)

	result, err := uc.lookupPractitioner(ctx(), "doc@test.com", "", "st-123")

	require.NoError(t, err)
	assert.Equal(t, expected, result)
	mockPrac.AssertNotCalled(t, "FindPractitionerByEmail", mock.Anything)
	mockPrac.AssertExpectations(t)
}

func TestLookupPatient_IdentifierMiss_FallsBackToEmail(t *testing.T) {
	// Legacy resources created before the identifier scheme carry no uid
	// identifier; the email fallback must still find them so
	// ensurePatientIdentifiers can attach the uid.
	mockPat := new(MockPatientFhirClient)
	uc := newTestUsecase(nil, mockPat, nil)
	expected := []fhir_dto.Patient{{ResourceType: "Patient"}}
	identifierQuery := constvars.FhirSupertokenSystemIdentifier + "|st-123"

	mockPat.On("FindPatientByIdentifier", ctx(), identifierQuery).Return([]fhir_dto.Patient{}, nil)
	mockPat.On("FindPatientByEmail", ctx(), "pat@test.com").Return(expected, nil)

	result, err := uc.lookupPatient(ctx(), "pat@test.com", "", "st-123")

	require.NoError(t, err)
	assert.Equal(t, expected, result)
	mockPat.AssertExpectations(t)
}

func TestLookupPractitioner_IdentifierMiss_FallsBackToEmail(t *testing.T) {
	mockPrac := new(MockPractitionerFhirClient)
	uc := newTestUsecase(mockPrac, nil, nil)
	expected := []fhir_dto.Practitioner{{ResourceType: "Practitioner"}}

	mockPrac.On("FindPractitionerByIdentifier", ctx(),
		constvars.FhirSupertokenSystemIdentifier, "st-123").Return([]fhir_dto.Practitioner{}, nil)
	mockPrac.On("FindPractitionerByEmail", ctx(), "doc@test.com").Return(expected, nil)

	result, err := uc.lookupPractitioner(ctx(), "doc@test.com", "", "st-123")

	require.NoError(t, err)
	assert.Equal(t, expected, result)
	mockPrac.AssertExpectations(t)
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
// createPractitionerRoleIfNotExists
// ---------------------------------------------------------------------------

func TestCreatePractitionerRoleIfNotExists_FoundByCode_ReturnsExisting(t *testing.T) {
	mockPR := new(MockPractitionerRoleFhirClient)
	uc := newTestUsecase(nil, nil, mockPR)

	existing := []fhir_dto.PractitionerRole{
		{
			ID:     "pr-role-1",
			Active: true,
			Code: []fhir_dto.CodeableConcept{{Coding: []fhir_dto.Coding{{
				System: constvars.FhirPractitionerRoleSystemHL7,
				Code:   constvars.FhirPractitionerRoleCodeResearcher,
			}}}},
		},
	}
	searchParams := contracts.PractitionerRoleSearchParams{
		PractitionerID: "prac-1",
		Code:           constvars.FhirPractitionerRoleSystemHL7 + "|" + constvars.FhirPractitionerRoleCodeResearcher,
	}
	mockPR.On("Search", ctx(), searchParams).Return(existing, nil)

	result, err := uc.createPractitionerRoleIfNotExists(ctx(), "prac-1", "",
		constvars.FhirPractitionerRoleSystemHL7, constvars.FhirPractitionerRoleCodeResearcher, constvars.FhirPractitionerRoleDisplayResearcher)

	require.NoError(t, err)
	assert.Equal(t, "pr-role-1", result.ID)
	mockPR.AssertNotCalled(t, "CreatePractitionerRole", mock.Anything)
	mockPR.AssertExpectations(t)
}

func TestCreatePractitionerRoleIfNotExists_ExistingRolesWithoutMatchingCode_CreatesNew(t *testing.T) {
	mockPR := new(MockPractitionerRoleFhirClient)
	uc := newTestUsecase(nil, nil, mockPR)

	// Blaze does not index the code search parameter, so the search returns every
	// role for the practitioner; the create-if-not-exists must filter client-side.
	existing := []fhir_dto.PractitionerRole{
		{ID: "pr-code-less", Active: false, Organization: fhir_dto.Reference{Reference: "Organization/org-1"}},
		{ID: "pr-other-code", Code: []fhir_dto.CodeableConcept{{Coding: []fhir_dto.Coding{{
			System: constvars.FhirPractitionerRoleSystemHL7,
			Code:   "clinician",
		}}}}},
	}
	searchParams := contracts.PractitionerRoleSearchParams{
		PractitionerID: "prac-1",
		Code:           constvars.FhirPractitionerRoleSystemHL7 + "|" + constvars.FhirPractitionerRoleCodeResearcher,
	}
	mockPR.On("Search", ctx(), searchParams).Return(existing, nil)

	created := &fhir_dto.PractitionerRole{ID: "pr-role-new", ResourceType: "PractitionerRole", Active: true}
	mockPR.On("CreatePractitionerRole", ctx(), mock.MatchedBy(func(r *fhir_dto.PractitionerRole) bool {
		return r.Active && len(r.Code) == 1 && r.Code[0].Coding[0].Code == constvars.FhirPractitionerRoleCodeResearcher
	})).Return(created, nil)

	result, err := uc.createPractitionerRoleIfNotExists(ctx(), "prac-1", "",
		constvars.FhirPractitionerRoleSystemHL7, constvars.FhirPractitionerRoleCodeResearcher, constvars.FhirPractitionerRoleDisplayResearcher)

	require.NoError(t, err)
	assert.Equal(t, "pr-role-new", result.ID)
	mockPR.AssertExpectations(t)
}

func TestCreatePractitionerRoleIfNotExists_NotFound_CreatesWithCodingAndOrg(t *testing.T) {
	mockPR := new(MockPractitionerRoleFhirClient)
	uc := newTestUsecase(nil, nil, mockPR)

	searchParams := contracts.PractitionerRoleSearchParams{
		PractitionerID: "prac-1",
		Code:           constvars.FhirPractitionerRoleSystemSnomed + "|" + constvars.FhirPractitionerRoleCodeAdministrativeStaff,
	}
	mockPR.On("Search", ctx(), searchParams).Return([]fhir_dto.PractitionerRole{}, nil)

	created := &fhir_dto.PractitionerRole{ID: "pr-role-new", ResourceType: "PractitionerRole", Active: true}
	mockPR.On("CreatePractitionerRole", ctx(), mock.MatchedBy(func(r *fhir_dto.PractitionerRole) bool {
		return r.ResourceType == "PractitionerRole" &&
			r.Active &&
			r.Practitioner.Reference == "Practitioner/prac-1" &&
			r.Organization.Reference == "Organization/org-1" &&
			len(r.Code) == 1 &&
			len(r.Code[0].Coding) == 1 &&
			r.Code[0].Coding[0].System == constvars.FhirPractitionerRoleSystemSnomed &&
			r.Code[0].Coding[0].Code == constvars.FhirPractitionerRoleCodeAdministrativeStaff &&
			r.Code[0].Coding[0].Display == constvars.FhirPractitionerRoleDisplayAdministrativeStaff
	})).Return(created, nil)

	result, err := uc.createPractitionerRoleIfNotExists(ctx(), "prac-1", "org-1",
		constvars.FhirPractitionerRoleSystemSnomed, constvars.FhirPractitionerRoleCodeAdministrativeStaff, constvars.FhirPractitionerRoleDisplayAdministrativeStaff)

	require.NoError(t, err)
	assert.Equal(t, "pr-role-new", result.ID)
	mockPR.AssertExpectations(t)
}

func TestCreatePractitionerRoleIfNotExists_SearchError_Propagates(t *testing.T) {
	mockPR := new(MockPractitionerRoleFhirClient)
	uc := newTestUsecase(nil, nil, mockPR)

	mockPR.On("Search", ctx(), mock.Anything).Return(nil, assert.AnError)

	_, err := uc.createPractitionerRoleIfNotExists(ctx(), "prac-1", "",
		constvars.FhirPractitionerRoleSystemHL7, constvars.FhirPractitionerRoleCodeResearcher, constvars.FhirPractitionerRoleDisplayResearcher)

	require.Error(t, err)
	assert.ErrorIs(t, err, assert.AnError)
	mockPR.AssertExpectations(t)
}

// ---------------------------------------------------------------------------
// InitializeNewUserFHIRResources (plan orchestrator)
// ---------------------------------------------------------------------------

func TestInitializeNewUserFHIRResources_ResearcherPlan(t *testing.T) {
	mockPrac := new(MockPractitionerFhirClient)
	mockPR := new(MockPractitionerRoleFhirClient)
	uc := newTestUsecase(mockPrac, nil, mockPR)
	uc.webhookForwardFn = stubWebhookForwarder()

	input := &contracts.InitializeNewUserFHIRResourcesInput{
		SuperTokenUserID: "st-123",
		Email:            "doc@test.com",
	}
	input.ToogleByRoles([]string{constvars.KonsulinRoleResearcher})

	createdPrac := &fhir_dto.Practitioner{ID: "prac-1", ResourceType: "Practitioner", Active: true}
	mockPrac.On("FindPractitionerByIdentifier", ctx(),
		constvars.FhirSupertokenSystemIdentifier, "st-123").Return([]fhir_dto.Practitioner{}, nil)
	mockPrac.On("FindPractitionerByEmail", ctx(), "doc@test.com").Return([]fhir_dto.Practitioner{}, nil)
	mockPrac.On("CreatePractitioner", ctx(), mock.Anything).Return(createdPrac, nil)

	searchParams := contracts.PractitionerRoleSearchParams{
		PractitionerID: "prac-1",
		Code:           constvars.FhirPractitionerRoleSystemHL7 + "|" + constvars.FhirPractitionerRoleCodeResearcher,
	}
	mockPR.On("Search", ctx(), searchParams).Return([]fhir_dto.PractitionerRole{}, nil)
	createdRole := &fhir_dto.PractitionerRole{ID: "role-1", ResourceType: "PractitionerRole", Active: true}
	mockPR.On("CreatePractitionerRole", ctx(), mock.Anything).Return(createdRole, nil)

	output, err := uc.InitializeNewUserFHIRResources(ctx(), input)

	require.NoError(t, err)
	assert.Equal(t, "prac-1", output.PractitionerID)
	assert.Equal(t, "", output.PatientID)
	assert.Equal(t, []string{"role-1"}, output.PractitionerRoleIDs)
	mockPrac.AssertExpectations(t)
	mockPR.AssertExpectations(t)
}

func TestInitializeNewUserFHIRResources_ClinicAdminAndResearcher_OrgLinked(t *testing.T) {
	mockPrac := new(MockPractitionerFhirClient)
	mockPR := new(MockPractitionerRoleFhirClient)
	uc := newTestUsecase(mockPrac, nil, mockPR)
	uc.webhookForwardFn = stubWebhookForwarder()

	input := &contracts.InitializeNewUserFHIRResourcesInput{
		SuperTokenUserID: "st-123",
		Email:            "admin@test.com",
		OrganizationID:   "org-1",
	}
	input.ToogleByRoles([]string{constvars.KonsulinRoleClinicAdmin, constvars.KonsulinRoleResearcher})

	createdPrac := &fhir_dto.Practitioner{ID: "prac-1", ResourceType: "Practitioner", Active: true}
	mockPrac.On("FindPractitionerByIdentifier", ctx(),
		constvars.FhirSupertokenSystemIdentifier, "st-123").Return([]fhir_dto.Practitioner{}, nil)
	mockPrac.On("FindPractitionerByEmail", ctx(), "admin@test.com").Return([]fhir_dto.Practitioner{}, nil)
	mockPrac.On("CreatePractitioner", ctx(), mock.Anything).Return(createdPrac, nil)

	adminParams := contracts.PractitionerRoleSearchParams{
		PractitionerID: "prac-1",
		Code:           constvars.FhirPractitionerRoleSystemSnomed + "|" + constvars.FhirPractitionerRoleCodeAdministrativeStaff,
	}
	researcherParams := contracts.PractitionerRoleSearchParams{
		PractitionerID: "prac-1",
		Code:           constvars.FhirPractitionerRoleSystemHL7 + "|" + constvars.FhirPractitionerRoleCodeResearcher,
	}
	mockPR.On("Search", ctx(), adminParams).Return([]fhir_dto.PractitionerRole{}, nil)
	mockPR.On("Search", ctx(), researcherParams).Return([]fhir_dto.PractitionerRole{}, nil)
	mockPR.On("CreatePractitionerRole", ctx(), mock.MatchedBy(func(r *fhir_dto.PractitionerRole) bool {
		return r.Active && r.Organization.Reference == "Organization/org-1"
	})).Return(
		&fhir_dto.PractitionerRole{ID: "role-admin", ResourceType: "PractitionerRole"},
		nil,
	).Once()
	mockPR.On("CreatePractitionerRole", ctx(), mock.MatchedBy(func(r *fhir_dto.PractitionerRole) bool {
		return r.Active && r.Organization.Reference == "Organization/org-1"
	})).Return(
		&fhir_dto.PractitionerRole{ID: "role-researcher", ResourceType: "PractitionerRole"},
		nil,
	).Once()

	output, err := uc.InitializeNewUserFHIRResources(ctx(), input)

	require.NoError(t, err)
	assert.Equal(t, "prac-1", output.PractitionerID)
	assert.ElementsMatch(t, []string{"role-admin", "role-researcher"}, output.PractitionerRoleIDs)
	mockPrac.AssertExpectations(t)
	mockPR.AssertExpectations(t)
}

func TestInitializeNewUserFHIRResources_PatientPlan_NoRoles(t *testing.T) {
	mockPrac := new(MockPractitionerFhirClient)
	mockPat := new(MockPatientFhirClient)
	mockPR := new(MockPractitionerRoleFhirClient)
	uc := newTestUsecase(mockPrac, mockPat, mockPR)
	uc.webhookForwardFn = stubWebhookForwarder()

	input := &contracts.InitializeNewUserFHIRResourcesInput{
		SuperTokenUserID: "st-123",
		Email:            "pat@test.com",
	}
	input.ToogleByRoles([]string{constvars.KonsulinRolePatient})

	createdPat := &fhir_dto.Patient{ID: "pat-1", ResourceType: "Patient", Active: true}
	mockPat.On("FindPatientByIdentifier", ctx(), constvars.FhirSupertokenSystemIdentifier+"|st-123").Return([]fhir_dto.Patient{}, nil)
	mockPat.On("FindPatientByEmail", ctx(), "pat@test.com").Return([]fhir_dto.Patient{}, nil)
	mockPat.On("CreatePatient", ctx(), mock.Anything).Return(createdPat, nil)

	output, err := uc.InitializeNewUserFHIRResources(ctx(), input)

	require.NoError(t, err)
	assert.Equal(t, "pat-1", output.PatientID)
	assert.Equal(t, "", output.PractitionerID)
	assert.Len(t, output.PractitionerRoleIDs, 0)
	mockPat.AssertExpectations(t)
	mockPR.AssertNotCalled(t, "Search", mock.Anything, mock.Anything)
}

func TestInitializeNewUserFHIRResources_DoubleInit_SinglePatient(t *testing.T) {
	// initializeFHIRForUser runs on every create-code AND consume-code (twice
	// per login). The uid-first lookup must make the second call converge on
	// the resource created by the first, never a duplicate Patient.
	mockPrac := new(MockPractitionerFhirClient)
	mockPat := new(MockPatientFhirClient)
	mockPR := new(MockPractitionerRoleFhirClient)
	uc := newTestUsecase(mockPrac, mockPat, mockPR)
	uc.webhookForwardFn = stubWebhookForwarder()

	input := &contracts.InitializeNewUserFHIRResourcesInput{
		SuperTokenUserID: "st-123",
		Email:            "pat@test.com",
	}
	input.ToogleByRoles([]string{constvars.KonsulinRolePatient})

	identifierQuery := constvars.FhirSupertokenSystemIdentifier + "|st-123"
	createdPat := &fhir_dto.Patient{ID: "pat-1", ResourceType: "Patient", Active: true}

	// First call: no existing resource anywhere -> create once.
	mockPat.On("FindPatientByIdentifier", ctx(), identifierQuery).Return([]fhir_dto.Patient{}, nil).Once()
	mockPat.On("FindPatientByEmail", ctx(), "pat@test.com").Return([]fhir_dto.Patient{}, nil).Once()
	mockPat.On("CreatePatient", ctx(), mock.Anything).Return(createdPat, nil).Once()
	// Second call: the uid identifier now resolves the existing resource.
	mockPat.On("FindPatientByIdentifier", ctx(), identifierQuery).Return(
		[]fhir_dto.Patient{{
			ID: "pat-1",
			Identifier: []fhir_dto.Identifier{
				{System: constvars.FhirSupertokenSystemIdentifier, Value: "st-123"},
			},
		}}, nil).Once()
	mockPat.On("UpdatePatient", ctx(), mock.Anything).Return(createdPat, nil).Once()

	out1, err := uc.InitializeNewUserFHIRResources(ctx(), input)
	require.NoError(t, err)
	out2, err := uc.InitializeNewUserFHIRResources(ctx(), input)
	require.NoError(t, err)

	assert.Equal(t, "pat-1", out1.PatientID)
	assert.Equal(t, "pat-1", out2.PatientID)
	mockPat.AssertNumberOfCalls(t, "CreatePatient", 1)
	mockPat.AssertExpectations(t)
}
