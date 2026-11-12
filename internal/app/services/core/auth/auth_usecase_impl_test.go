package auth

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/exceptions"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

// TestCreateMagicLink_PhoneInStandaloneReturnsGatewayNotConfigured verifies
// T41: a phone magic link in standalone mode (gateway not configured) fails
// fast with 503 and dev_message GATEWAY_NOT_CONFIGURED, without touching
// SuperTokens.
func TestCreateMagicLink_PhoneInStandaloneReturnsGatewayNotConfigured(t *testing.T) {
	uc := &authUsecase{
		InternalConfig: &config.InternalConfig{},
		Log:            zap.NewNop(),
	}

	err := uc.CreateMagicLink(context.Background(), &requests.SupertokenPasswordlessCreateMagicLink{
		Phone: "6281234567890",
	})
	require.Error(t, err, "phone magic link in standalone mode must be rejected")

	var customErr *exceptions.CustomError
	require.True(t, errors.As(err, &customErr), "error must be a CustomError")
	assert.Equal(t, http.StatusServiceUnavailable, customErr.StatusCode,
		"standalone phone magic link must answer 503")
	assert.Equal(t, "GATEWAY_NOT_CONFIGURED", customErr.DevMessage,
		"dev_message must be the GATEWAY_NOT_CONFIGURED code")
	assert.Contains(t, customErr.ClientMessage, "WhatsApp login requires the federation gateway",
		"client message must say WhatsApp login requires the federation gateway")
}

func TestLogErrorAndReturn(t *testing.T) {
	// Arrange: capture log output
	core, observedLogs := observer.New(zapcore.InfoLevel)
	logger := zap.New(core)
	expectedErr := errors.New("something went wrong")

	// Act
	err := logErrorAndReturn(logger, "req-123", "test error message", expectedErr)

	// Assert: returns the same error
	assert.Equal(t, expectedErr, err)

	// Assert: logs exactly one error entry with request ID
	logs := observedLogs.TakeAll()
	require.Len(t, logs, 1)
	entry := logs[0]
	assert.Equal(t, zapcore.ErrorLevel, entry.Level)
	assert.Equal(t, "test error message", entry.Message)
	ctxMap := entry.ContextMap()
	assert.Equal(t, "req-123", ctxMap[constvars.LoggingRequestIDKey])
	assert.Equal(t, expectedErr.Error(), ctxMap["error"])
}

func TestLogErrorAndReturn_NilError(t *testing.T) {
	// Arrange: capture log output
	core, observedLogs := observer.New(zapcore.InfoLevel)
	logger := zap.New(core)

	// Act
	err := logErrorAndReturn(logger, "req-456", "nil error test", nil)

	// Assert: returns nil
	assert.NoError(t, err)

	// Assert: still logs the message
	logs := observedLogs.TakeAll()
	require.Len(t, logs, 1)
	assert.Equal(t, "nil error test", logs[0].Message)
}

// MockUserFHIRInitializer implements contracts.UserFHIRInitializer for testing.
type MockUserFHIRInitializer struct {
	mock.Mock
}

func (m *MockUserFHIRInitializer) InitializeNewUserFHIRResources(ctx context.Context, input *contracts.InitializeNewUserFHIRResourcesInput) (*contracts.InitializeNewUserFHIRResourcesOutput, error) {
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

	mockUserFHIRInitializer := new(MockUserFHIRInitializer)
	uc := &authUsecase{
		UserFHIRInitializer: mockUserFHIRInitializer,
		Log:                 logger,
	}

	expectedErr := errors.New("FHIR server timeout")
	mockUserFHIRInitializer.On("InitializeNewUserFHIRResources", mock.Anything, mock.AnythingOfType("*contracts.InitializeNewUserFHIRResourcesInput")).
		Return(nil, expectedErr)

	start := time.Now()

	// Act
	_, err := initializeMagicLinkFHIR(context.Background(), initializeMagicLinkFHIRInput{
		Uc:               uc,
		RequestID:        "req-abc-123",
		SuperTokenUserID: "st-user-1",
		Roles:            []string{"patient"},
		Email:            "test@example.com",
		Phone:            "",
		Start:            start,
	})

	// Assert: error is propagated
	require.Error(t, err)
	assert.Equal(t, expectedErr, err)
	mockUserFHIRInitializer.AssertExpectations(t)

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

	mockUserFHIRInitializer := new(MockUserFHIRInitializer)
	uc := &authUsecase{
		UserFHIRInitializer: mockUserFHIRInitializer,
		Log:                 logger,
	}

	expectedOutput := &contracts.InitializeNewUserFHIRResourcesOutput{
		PatientID:           "pat-1",
		PractitionerID:      "prac-1",
		PractitionerRoleIDs: []string{"pr-role-1"},
	}
	mockUserFHIRInitializer.On("InitializeNewUserFHIRResources", mock.Anything, mock.AnythingOfType("*contracts.InitializeNewUserFHIRResourcesInput")).
		Return(expectedOutput, nil)

	start := time.Now()

	// Act
	output, err := initializeMagicLinkFHIR(context.Background(), initializeMagicLinkFHIRInput{
		Uc:               uc,
		RequestID:        "req-abc-456",
		SuperTokenUserID: "st-user-2",
		Roles:            []string{"patient", "practitioner"},
		Email:            "doctor@example.com",
		Phone:            "",
		Start:            start,
	})

	// Assert: no error, output returned
	require.NoError(t, err)
	assert.Equal(t, expectedOutput, output)
	mockUserFHIRInitializer.AssertExpectations(t)

	// Assert: no error logs were emitted
	logs := observedLogs.TakeAll()
	assert.Len(t, logs, 0, "should not log anything on success")
}

func TestInitializeMagicLinkFHIR_PhoneUser(t *testing.T) {
	// Arrange
	core, observedLogs := observer.New(zapcore.InfoLevel)
	logger := zap.New(core)

	mockUserFHIRInitializer := new(MockUserFHIRInitializer)
	uc := &authUsecase{
		UserFHIRInitializer: mockUserFHIRInitializer,
		Log:                 logger,
	}

	expectedErr := errors.New("phone FHIR error")
	mockUserFHIRInitializer.On("InitializeNewUserFHIRResources", mock.Anything, mock.AnythingOfType("*contracts.InitializeNewUserFHIRResourcesInput")).
		Return(nil, expectedErr)

	start := time.Now()

	// Act
	_, err := initializeMagicLinkFHIR(context.Background(), initializeMagicLinkFHIRInput{
		Uc:               uc,
		RequestID:        "req-phone-789",
		SuperTokenUserID: "st-user-3",
		Roles:            []string{"practitioner"},
		Email:            "",
		Phone:            "6281234567890",
		Start:            start,
	})

	// Assert
	require.Error(t, err)
	assert.Equal(t, expectedErr, err)
	mockUserFHIRInitializer.AssertExpectations(t)

	// Assert: error logged with phone user context
	logs := observedLogs.TakeAll()
	require.Len(t, logs, 1)
	entry := logs[0]
	assert.Equal(t, zapcore.ErrorLevel, entry.Level)
	ctxMap := entry.ContextMap()
	assert.Equal(t, "req-phone-789", ctxMap[constvars.LoggingRequestIDKey])
	assert.Equal(t, "FHIR initialization", ctxMap[constvars.LoggingErrorTypeKey])
}
