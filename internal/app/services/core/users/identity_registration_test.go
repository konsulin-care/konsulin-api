package users

import (
	"context"
	"strings"
	"testing"

	"github.com/konsulin-care/konsulin-api/internal/app/contracts"
	"github.com/konsulin-care/konsulin-api/internal/pkg/constvars"
	"github.com/konsulin-care/konsulin-api/internal/pkg/fhir_dto"

	"github.com/konsulin-care/gateway-sdk-go/identity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

// identityTestPepper is the fixed 32-byte test pepper the identity hashes are
// keyed with — test-only material, never a production pepper.
const identityTestPepper = "0123456789abcdef0123456789abcdef"

// useIdentityTestPepper isolates the pepper loader: no sealed artifact, the
// APP_PEPPER fallback only, keyed with the fixed test pepper.
// Parameters: t is the test harness.
// Returns nothing; fails the test on env setup error.
func useIdentityTestPepper(t *testing.T) {
	t.Helper()
	t.Setenv("APP_DATA_DIR", t.TempDir())
	t.Setenv("APP_PEPPER", identityTestPepper)
}

// identityRegisterCall records one IdentityClient.Register invocation.
type identityRegisterCall struct {
	resourceType    string
	localResourceID string
	emailHash       string
	phoneHash       string
}

// fakeIdentityClient is an in-memory IdentityClient: it records Register
// calls and answers with the configured result, so no gateway I/O runs.
type fakeIdentityClient struct {
	enabled bool
	err     error
	calls   []identityRegisterCall
}

// Register records the invocation and returns the configured result.
func (f *fakeIdentityClient) Register(_ context.Context, resourceType, localResourceID, emailHash, phoneHash string) (string, error) {
	f.calls = append(f.calls, identityRegisterCall{
		resourceType:    resourceType,
		localResourceID: localResourceID,
		emailHash:       emailHash,
		phoneHash:       phoneHash,
	})
	if f.err != nil {
		return "", f.err
	}
	return "global-key-1", nil
}

// Resolve is unused by the signup registration; it answers no candidates.
func (f *fakeIdentityClient) Resolve(context.Context, string, string) ([]contracts.IdentityCandidate, error) {
	return nil, nil
}

// Link is unused by the signup registration; it answers not-linked.
func (f *fakeIdentityClient) Link(context.Context, string, string) (bool, error) {
	return false, nil
}

// Enabled reports the configured gateway availability (false = standalone).
func (f *fakeIdentityClient) Enabled() bool { return f.enabled }

// newIdentitySignupUsecase builds a userUsecase over the patient-only FHIR
// fixture (no existing Patient, then one created as pat-1) with the given
// identity client. It returns the usecase, the patient mock (expectations
// already set) and the recorded error-level log entries.
// Parameters: t is the test harness, identityClient is the injected gateway
// identity client (nil models missing wiring).
// Returns the usecase, the patient mock and the observed log entries.
func newIdentitySignupUsecase(t *testing.T, identityClient IdentityClient) (*userUsecase, *MockPatientFhirClient, *observer.ObservedLogs) {
	t.Helper()
	obsCore, obs := observer.New(zap.ErrorLevel)
	mockPat := new(MockPatientFhirClient)
	identifierQuery := constvars.FhirSupertokenSystemIdentifier + "|st-123"
	mockPat.On("FindPatientByIdentifier", ctx(), identifierQuery).Return([]fhir_dto.Patient{}, nil)
	mockPat.On("FindPatientByEmail", ctx(), "pat@example.com").Return([]fhir_dto.Patient{}, nil)
	mockPat.On("CreatePatient", ctx(), mock.Anything).Return(&fhir_dto.Patient{
		ID: "pat-1", ResourceType: "Patient", Active: true,
	}, nil)
	uc := &userUsecase{
		UserFHIRInitializerDeps: UserFHIRInitializerDeps{
			PatientFhirClient: mockPat,
			Logger:            zap.New(obsCore),
			IdentityClient:    identityClient,
		},
	}
	return uc, mockPat, obs
}

// identitySignupInput is the patient-plan signup input carrying both contact
// fields and the supertoken uid.
// Parameters: none.
// Returns the input with the patient role toggled on.
func identitySignupInput() *contracts.InitializeNewUserFHIRResourcesInput {
	input := &contracts.InitializeNewUserFHIRResourcesInput{
		SuperTokenUserID: "st-123",
		Email:            "pat@example.com",
		Phone:            "6281234567890",
	}
	input.ToogleByRoles([]string{constvars.KonsulinRolePatient})
	return input
}

// TestInitializeNewUserFHIRResources_RegistersHashedIdentities covers T44
// assertion 1: initializing a new user registers its peppered email and
// phone hashes with the gateway identity plane, one Register per created
// subject, never plaintext. The phone-only practitioner case pins the "N/A"
// marker for the non-applicable field.
// Parameters: t is the test harness.
// Returns nothing; fails on any deviation.
func TestInitializeNewUserFHIRResources_RegistersHashedIdentities(t *testing.T) {
	useIdentityTestPepper(t)
	pepper := []byte(identityTestPepper)

	t.Run("patient plan registers the created patient", func(t *testing.T) {
		fake := &fakeIdentityClient{enabled: true}
		uc, mockPat, obs := newIdentitySignupUsecase(t, fake)

		output, err := uc.InitializeNewUserFHIRResources(ctx(), identitySignupInput())

		require.NoError(t, err)
		require.NotNil(t, output)
		assert.Equal(t, "pat-1", output.PatientID, "the FHIR initialization must still create the patient")
		assert.Zero(t, obs.Len(), "a successful registration must not log errors")

		require.Len(t, fake.calls, 1, "exactly one identity registration must happen")
		call := fake.calls[0]
		assert.Equal(t, "patient", call.resourceType, "must register the patient subject")
		assert.Equal(t, "Patient/pat-1", call.localResourceID, "must register the created patient reference")

		wantEmailHash, err := identity.HashEmail("pat@example.com", pepper)
		require.NoError(t, err)
		wantPhoneHash, err := identity.HashPhone("6281234567890", pepper)
		require.NoError(t, err)
		assert.Equal(t, wantEmailHash, call.emailHash, "must register the peppered email hash")
		assert.Equal(t, wantPhoneHash, call.phoneHash, "must register the peppered phone hash")
		assert.NotContains(t, call.emailHash, "pat@example.com", "plaintext email must never be registered")
		assert.NotContains(t, call.phoneHash, "6281234567890", "plaintext phone must never be registered")
		mockPat.AssertExpectations(t)
	})

	t.Run("practitioner plan registers the created practitioner", func(t *testing.T) {
		fake := &fakeIdentityClient{enabled: true}
		obsCore, _ := observer.New(zap.ErrorLevel)
		mockPrac := new(MockPractitionerFhirClient)
		mockPrac.On("FindPractitionerByIdentifier", ctx(), constvars.FhirSupertokenSystemIdentifier, "st-123").Return([]fhir_dto.Practitioner{}, nil)
		mockPrac.On("FindPractitionerByPhone", ctx(), "6281234567890").Return([]fhir_dto.Practitioner{}, nil)
		mockPrac.On("CreatePractitioner", ctx(), mock.Anything).Return(&fhir_dto.Practitioner{
			ID: "prac-1", ResourceType: "Practitioner", Active: true,
		}, nil)
		uc := &userUsecase{
			UserFHIRInitializerDeps: UserFHIRInitializerDeps{
				PractitionerFhirClient: mockPrac,
				Logger:                 zap.New(obsCore),
				IdentityClient:         fake,
			},
		}

		input := &contracts.InitializeNewUserFHIRResourcesInput{
			SuperTokenUserID: "st-123",
			Phone:            "6281234567890",
		}
		input.ToogleByRoles([]string{constvars.KonsulinRolePractitioner})

		output, err := uc.InitializeNewUserFHIRResources(ctx(), input)

		require.NoError(t, err)
		require.NotNil(t, output)
		assert.Equal(t, "prac-1", output.PractitionerID)

		require.Len(t, fake.calls, 1, "exactly one identity registration must happen")
		call := fake.calls[0]
		assert.Equal(t, "practitioner", call.resourceType, "must register the practitioner subject")
		assert.Equal(t, "Practitioner/prac-1", call.localResourceID, "must register the created practitioner reference")
		assert.Equal(t, identity.NAMarker, call.emailHash,
			"a missing field must register the N/A hashfmt marker")
		wantPhoneHash, err := identity.HashPhone("6281234567890", pepper)
		require.NoError(t, err)
		assert.Equal(t, wantPhoneHash, call.phoneHash, "must register the peppered phone hash")
		mockPrac.AssertExpectations(t)
	})
}

// TestInitializeNewUserFHIRResources_IdentityRegistrationNeverBreaksSignup
// covers T44 assertion 2: a gateway registration failure is logged and the
// signup still succeeds, and nothing is sent in standalone mode (disabled
// gateway adapter or missing wiring).
// Parameters: t is the test harness.
// Returns nothing; fails on any deviation.
func TestInitializeNewUserFHIRResources_IdentityRegistrationNeverBreaksSignup(t *testing.T) {
	t.Run("registration failure is logged and signup succeeds", func(t *testing.T) {
		useIdentityTestPepper(t)
		fake := &fakeIdentityClient{enabled: true, err: assert.AnError}
		uc, mockPat, obs := newIdentitySignupUsecase(t, fake)

		output, err := uc.InitializeNewUserFHIRResources(ctx(), identitySignupInput())

		require.NoError(t, err, "an identity registration failure must never fail the signup")
		require.NotNil(t, output)
		assert.Equal(t, "pat-1", output.PatientID, "the FHIR initialization result must be kept")
		require.Len(t, fake.calls, 1, "the registration must have been attempted")

		logged := obs.Filter(func(e observer.LoggedEntry) bool {
			return strings.Contains(e.Message, "identity registration")
		})
		assert.Positive(t, logged.Len(), "the registration failure must be logged")
		mockPat.AssertExpectations(t)
	})

	t.Run("standalone mode sends nothing", func(t *testing.T) {
		useIdentityTestPepper(t)
		fake := &fakeIdentityClient{enabled: false}
		uc, mockPat, _ := newIdentitySignupUsecase(t, fake)

		output, err := uc.InitializeNewUserFHIRResources(ctx(), identitySignupInput())

		require.NoError(t, err, "standalone mode must not fail the signup")
		require.NotNil(t, output)
		assert.Equal(t, "pat-1", output.PatientID)
		assert.Empty(t, fake.calls, "nothing may be sent in standalone mode")
		mockPat.AssertExpectations(t)
	})

	t.Run("missing wiring sends nothing", func(t *testing.T) {
		useIdentityTestPepper(t)
		uc, mockPat, _ := newIdentitySignupUsecase(t, nil)

		output, err := uc.InitializeNewUserFHIRResources(ctx(), identitySignupInput())

		require.NoError(t, err, "missing wiring must not fail the signup")
		require.NotNil(t, output)
		assert.Equal(t, "pat-1", output.PatientID)
		mockPat.AssertExpectations(t)
	})
}
