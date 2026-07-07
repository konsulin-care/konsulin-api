package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/pkg/dto/responses"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/constvars"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

// MockUserUsecase implements contracts.UserUsecase for testing.
type MockUserUsecase struct {
	mock.Mock
}

func (m *MockUserUsecase) GetUserProfileBySession(ctx context.Context, sessionData string) (*responses.UserProfile, error) {
	args := m.Called(ctx, sessionData)
	var out *responses.UserProfile
	if v := args.Get(0); v != nil {
		out = v.(*responses.UserProfile)
	}
	return out, args.Error(1)
}

func (m *MockUserUsecase) UpdateUserProfileBySession(ctx context.Context, sessionData string, request *requests.UpdateProfile) (*responses.UpdateUserProfile, error) {
	args := m.Called(ctx, sessionData, request)
	var out *responses.UpdateUserProfile
	if v := args.Get(0); v != nil {
		out = v.(*responses.UpdateUserProfile)
	}
	return out, args.Error(1)
}

func (m *MockUserUsecase) DeleteUserBySession(ctx context.Context, sessionData string) error {
	args := m.Called(ctx, sessionData)
	return args.Error(0)
}

func (m *MockUserUsecase) DeactivateUserBySession(ctx context.Context, sessionData string) error {
	args := m.Called(ctx, sessionData)
	return args.Error(0)
}

func (m *MockUserUsecase) InitializeNewUserFHIRResources(ctx context.Context, input *contracts.InitializeNewUserFHIRResourcesInput) (*contracts.InitializeNewUserFHIRResourcesOutput, error) {
	args := m.Called(ctx, input)
	var out *contracts.InitializeNewUserFHIRResourcesOutput
	if v := args.Get(0); v != nil {
		out = v.(*contracts.InitializeNewUserFHIRResourcesOutput)
	}
	return out, args.Error(1)
}

func TestInitializeMagicLinkFHIR_LogsError(t *testing.T) {
	// Arrange: capture log output
	core, observedLogs := observer.New(zapcore.InfoLevel)
	logger := zap.New(core)

	mockUserUsecase := new(MockUserUsecase)
	uc := &authUsecase{
		UserUsecase: mockUserUsecase,
		Log:         logger,
	}

	expectedErr := errors.New("FHIR server timeout")
	mockUserUsecase.On("InitializeNewUserFHIRResources", mock.Anything, mock.AnythingOfType("*contracts.InitializeNewUserFHIRResourcesInput")).
		Return(nil, expectedErr)

	start := time.Now()

	// Act
	_, err := initializeMagicLinkFHIR(
		context.Background(),
		uc,
		"req-abc-123",
		"st-user-1",
		[]string{"patient"},
		"test@example.com",
		"",
		start,
	)

	// Assert: error is propagated
	require.Error(t, err)
	assert.Equal(t, expectedErr, err)
	mockUserUsecase.AssertExpectations(t)

	// Assert: error was logged with correct fields
	logs := observedLogs.TakeAll()
	require.Len(t, logs, 1)
	entry := logs[0]

	assert.Equal(t, zapcore.ErrorLevel, entry.Level)
	assert.Equal(t, "Failed to initialize FHIR resources during magic link creation", entry.Message)

	ctxMap := entry.ContextMap()
	assert.Equal(t, "req-abc-123", ctxMap[constvars.LoggingRequestIDKey])
	assert.Equal(t, "FHIR initialization", ctxMap[constvars.LoggingErrorTypeKey])
	assert.Contains(t, ctxMap, constvars.LoggingDurationKey)
	assert.Equal(t, expectedErr.Error(), ctxMap["error"])
}

func TestInitializeMagicLinkFHIR_Success(t *testing.T) {
	// Arrange
	core, observedLogs := observer.New(zapcore.InfoLevel)
	logger := zap.New(core)

	mockUserUsecase := new(MockUserUsecase)
	uc := &authUsecase{
		UserUsecase: mockUserUsecase,
		Log:         logger,
	}

	expectedOutput := &contracts.InitializeNewUserFHIRResourcesOutput{
		PatientID:      "pat-1",
		PractitionerID: "prac-1",
		PersonID:       "per-1",
	}
	mockUserUsecase.On("InitializeNewUserFHIRResources", mock.Anything, mock.AnythingOfType("*contracts.InitializeNewUserFHIRResourcesInput")).
		Return(expectedOutput, nil)

	start := time.Now()

	// Act
	output, err := initializeMagicLinkFHIR(
		context.Background(),
		uc,
		"req-abc-456",
		"st-user-2",
		[]string{"patient", "practitioner"},
		"doctor@example.com",
		"",
		start,
	)

	// Assert: no error, output returned
	require.NoError(t, err)
	assert.Equal(t, expectedOutput, output)
	mockUserUsecase.AssertExpectations(t)

	// Assert: no error logs were emitted
	logs := observedLogs.TakeAll()
	assert.Len(t, logs, 0, "should not log anything on success")
}

func TestInitializeMagicLinkFHIR_PhoneUser(t *testing.T) {
	// Arrange
	core, observedLogs := observer.New(zapcore.InfoLevel)
	logger := zap.New(core)

	mockUserUsecase := new(MockUserUsecase)
	uc := &authUsecase{
		UserUsecase: mockUserUsecase,
		Log:         logger,
	}

	expectedErr := errors.New("phone FHIR error")
	mockUserUsecase.On("InitializeNewUserFHIRResources", mock.Anything, mock.AnythingOfType("*contracts.InitializeNewUserFHIRResourcesInput")).
		Return(nil, expectedErr)

	start := time.Now()

	// Act
	_, err := initializeMagicLinkFHIR(
		context.Background(),
		uc,
		"req-phone-789",
		"st-user-3",
		[]string{"practitioner"},
		"",
		"6281234567890",
		start,
	)

	// Assert
	require.Error(t, err)
	assert.Equal(t, expectedErr, err)
	mockUserUsecase.AssertExpectations(t)

	// Assert: error logged with phone user context
	logs := observedLogs.TakeAll()
	require.Len(t, logs, 1)
	entry := logs[0]
	assert.Equal(t, zapcore.ErrorLevel, entry.Level)
	ctxMap := entry.ContextMap()
	assert.Equal(t, "req-phone-789", ctxMap[constvars.LoggingRequestIDKey])
	assert.Equal(t, "FHIR initialization", ctxMap[constvars.LoggingErrorTypeKey])
}
