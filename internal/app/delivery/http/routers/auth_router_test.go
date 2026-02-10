package routers

import (
	"bytes"
	"context"
	"encoding/json"
	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/app/delivery/http/controllers"
	"konsulin-service/internal/app/delivery/http/middlewares"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/requests"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/supertokens/supertokens-golang/recipe/passwordless/plessmodels"
	"go.uber.org/zap"
)

type MockAuthUsecase struct {
	mock.Mock
}

func makeUserExistsOutput(email string) *contracts.CheckUserExistsOutput {
	return &contracts.CheckUserExistsOutput{
		SupertokenUser: &plessmodels.User{
			Email: &email,
		},
		PatientIds:      []string{},
		PractitionerIds: []string{},
	}
}

func (m *MockAuthUsecase) InitializeSupertoken() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockAuthUsecase) CreateMagicLink(ctx context.Context, request *requests.SupertokenPasswordlessCreateMagicLink) error {
	args := m.Called(ctx, request)
	return args.Error(0)
}

func (m *MockAuthUsecase) CreateAnonymousSession(ctx context.Context, existingToken string, forceNew bool) (*contracts.AnonymousSessionResult, error) {
	args := m.Called(ctx, existingToken, forceNew)
	var out *contracts.AnonymousSessionResult
	if args.Get(0) != nil {
		out = args.Get(0).(*contracts.AnonymousSessionResult)
	}
	return out, args.Error(1)
}

func (m *MockAuthUsecase) LogoutUser(ctx context.Context, sessionData string) error {
	args := m.Called(ctx, sessionData)
	return args.Error(0)
}

func (m *MockAuthUsecase) CheckUserExists(ctx context.Context, email string) (*contracts.CheckUserExistsOutput, error) {
	args := m.Called(ctx, email)
	var out *contracts.CheckUserExistsOutput
	if v := args.Get(0); v != nil {
		out = v.(*contracts.CheckUserExistsOutput)
	}
	return out, args.Error(1)
}

func (m *MockAuthUsecase) CheckUserExistsByPhone(ctx context.Context, phone string) (*contracts.CheckUserExistsOutput, error) {
	args := m.Called(ctx, phone)
	var out *contracts.CheckUserExistsOutput
	if v := args.Get(0); v != nil {
		out = v.(*contracts.CheckUserExistsOutput)
	}
	return out, args.Error(1)
}

func TestAuthRouter_MagicLinkEndpoint(t *testing.T) {
	logger := zap.NewNop()

	testAPIKey := "test-superadmin-api-key-12345"
	internalConfig := &config.InternalConfig{
		App: config.App{
			SuperadminAPIKey: testAPIKey,
		},
	}

	mockAuthUsecase := new(MockAuthUsecase)

	authController := &controllers.AuthController{Log: logger, AuthUsecase: mockAuthUsecase}

	middlewareInstance := &middlewares.Middlewares{
		Log:            logger,
		InternalConfig: internalConfig,
	}

	router := chi.NewRouter()
	// Inject a request ID so controllers pass their preliminary checks without
	// needing the real RequestID middleware stack.
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), constvars.CONTEXT_REQUEST_ID_KEY, "test-request-id")
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	attachAuthRoutes(router, middlewareInstance, authController)

	t.Run("MagicLink with Valid API Key", func(t *testing.T) {
		mockAuthUsecase.ExpectedCalls = nil
		mockAuthUsecase.Calls = nil

		mockAuthUsecase.On("CheckUserExists", mock.Anything, "test@example.com").Return(nil, nil)
		mockAuthUsecase.On("CreateMagicLink", mock.Anything, mock.AnythingOfType("*requests.SupertokenPasswordlessCreateMagicLink")).Return(nil)

		requestBody := requests.SupertokenPasswordlessCreateMagicLink{
			Email: "test@example.com",
			Roles: []string{"Practitioner"},
		}
		jsonBody, _ := json.Marshal(requestBody)

		req := httptest.NewRequest("POST", "/magiclink", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-api-key", testAPIKey)

		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code, "should return 200 OK for valid API key")
		mockAuthUsecase.AssertExpectations(t)
	})

	t.Run("MagicLink with Valid API Key (Phone)", func(t *testing.T) {
		mockAuthUsecase.ExpectedCalls = nil
		mockAuthUsecase.Calls = nil
		mockAuthUsecase.On("CheckUserExistsByPhone", mock.Anything, "628111234567").Return(nil, nil)
		mockAuthUsecase.On("CreateMagicLink", mock.Anything, mock.AnythingOfType("*requests.SupertokenPasswordlessCreateMagicLink")).Return(nil)

		requestBody := requests.SupertokenPasswordlessCreateMagicLink{
			Phone: "628111234567",
			Roles: []string{"Practitioner"},
		}
		jsonBody, _ := json.Marshal(requestBody)

		req := httptest.NewRequest("POST", "/magiclink", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-api-key", testAPIKey)

		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code, "should return 200 OK for valid API key (phone)")
		mockAuthUsecase.AssertExpectations(t)
	})

	t.Run("MagicLink without API Key", func(t *testing.T) {

		requestBody := requests.SupertokenPasswordlessCreateMagicLink{
			Email: "test@example.com",
			Roles: []string{"Practitioner"},
		}
		jsonBody, _ := json.Marshal(requestBody)

		req := httptest.NewRequest("POST", "/magiclink", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code, "should return 401 Unauthorized for missing API key")

		mockAuthUsecase.AssertNotCalled(t, "CreateMagicLink")
	})

	t.Run("MagicLink with Invalid API Key", func(t *testing.T) {

		requestBody := requests.SupertokenPasswordlessCreateMagicLink{
			Email: "test@example.com",
			Roles: []string{"Practitioner"},
		}
		jsonBody, _ := json.Marshal(requestBody)

		req := httptest.NewRequest("POST", "/magiclink", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-api-key", "invalid-api-key")

		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code, "should return 401 Unauthorized for invalid API key")

		mockAuthUsecase.AssertNotCalled(t, "CreateMagicLink")
	})

	t.Run("MagicLink with Empty API Key", func(t *testing.T) {

		requestBody := requests.SupertokenPasswordlessCreateMagicLink{
			Email: "test@example.com",
			Roles: []string{"Practitioner"},
		}
		jsonBody, _ := json.Marshal(requestBody)

		req := httptest.NewRequest("POST", "/magiclink", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-api-key", "")

		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code, "should return 401 Unauthorized for empty API key")

		mockAuthUsecase.AssertNotCalled(t, "CreateMagicLink")
	})

	t.Run("Anonymous Session without API Key - Should Work", func(t *testing.T) {

		mockAuthUsecase.On("CreateAnonymousSession", mock.Anything, mock.Anything, mock.Anything).
			Return(&contracts.AnonymousSessionResult{Token: "test-session-handle", GuestID: "test-guest-id", IsNew: true}, nil)

		req := httptest.NewRequest("POST", "/anonymous-session", nil)

		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code, "should return 200 OK for anonymous session without API key")
		mockAuthUsecase.AssertExpectations(t)
	})
}

func TestAuthRouter_ContextPropagation(t *testing.T) {
	logger := zap.NewNop()

	testAPIKey := "test-superadmin-api-key-12345"
	internalConfig := &config.InternalConfig{
		App: config.App{
			SuperadminAPIKey: testAPIKey,
		},
	}

	mockAuthUsecase := new(MockAuthUsecase)

	authController := &controllers.AuthController{Log: logger, AuthUsecase: mockAuthUsecase}

	middlewareInstance := &middlewares.Middlewares{
		Log:            logger,
		InternalConfig: internalConfig,
	}

	router := chi.NewRouter()
	// Inject a request ID so controllers pass their preliminary checks without
	// needing the real RequestID middleware stack.
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), constvars.CONTEXT_REQUEST_ID_KEY, "test-request-id")
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	attachAuthRoutes(router, middlewareInstance, authController)

	t.Run("Context Values Propagated to Controller", func(t *testing.T) {
		mockAuthUsecase.ExpectedCalls = nil
		mockAuthUsecase.Calls = nil
		mockAuthUsecase.On("CheckUserExists", mock.Anything, "test@example.com").Return(nil, nil)

		mockAuthUsecase.On("CreateMagicLink", mock.MatchedBy(func(ctx context.Context) bool {

			apiKeyAuth, ok := ctx.Value(middlewares.ContextAPIKeyAuth).(bool)
			if !ok || !apiKeyAuth {
				return false
			}

			roles, ok := ctx.Value(constvars.CONTEXT_FHIR_ROLE).([]string)
			if !ok || len(roles) != 1 || roles[0] != constvars.KonsulinRoleSuperadmin {
				return false
			}

			uid, ok := ctx.Value(constvars.CONTEXT_UID).(string)
			if !ok || uid != "api-key-superadmin" {
				return false
			}

			return true
		}), mock.AnythingOfType("*requests.SupertokenPasswordlessCreateMagicLink")).Return(nil)

		requestBody := requests.SupertokenPasswordlessCreateMagicLink{
			Email: "test@example.com",
			Roles: []string{"Practitioner"},
		}
		jsonBody, _ := json.Marshal(requestBody)

		req := httptest.NewRequest("POST", "/magiclink", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-api-key", testAPIKey)

		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code, "should return 200 OK")
		mockAuthUsecase.AssertExpectations(t)
	})
}

func TestAuthRouter_ErrorHandling(t *testing.T) {
	logger := zap.NewNop()

	testAPIKey := "test-superadmin-api-key-12345"
	internalConfig := &config.InternalConfig{
		App: config.App{
			SuperadminAPIKey: testAPIKey,
		},
	}

	mockAuthUsecase := new(MockAuthUsecase)

	authController := &controllers.AuthController{Log: logger, AuthUsecase: mockAuthUsecase}

	middlewareInstance := &middlewares.Middlewares{
		Log:            logger,
		InternalConfig: internalConfig,
	}

	router := chi.NewRouter()
	// Inject a request ID so controllers pass their preliminary checks without
	// needing the real RequestID middleware stack.
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), constvars.CONTEXT_REQUEST_ID_KEY, "test-request-id")
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	attachAuthRoutes(router, middlewareInstance, authController)

	t.Run("Invalid JSON Body", func(t *testing.T) {
		mockAuthUsecase.ExpectedCalls = nil
		mockAuthUsecase.Calls = nil

		req := httptest.NewRequest("POST", "/magiclink", bytes.NewBufferString("invalid json"))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-api-key", testAPIKey)

		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code, "should return 400 Bad Request for invalid JSON")

		mockAuthUsecase.AssertNotCalled(t, "CreateMagicLink")
	})

	t.Run("Missing Email Field", func(t *testing.T) {
		mockAuthUsecase.ExpectedCalls = nil
		mockAuthUsecase.Calls = nil

		requestBody := map[string]interface{}{
			"roles": []string{"Practitioner"},
		}
		jsonBody, _ := json.Marshal(requestBody)

		req := httptest.NewRequest("POST", "/magiclink", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-api-key", testAPIKey)

		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code, "should return 400 Bad Request for missing email")

		mockAuthUsecase.AssertNotCalled(t, "CreateMagicLink")
	})

	t.Run("Missing Roles Field", func(t *testing.T) {
		mockAuthUsecase.ExpectedCalls = nil
		mockAuthUsecase.Calls = nil
		mockAuthUsecase.On("CheckUserExists", mock.Anything, "test@example.com").Return(nil, nil)

		requestBody := map[string]interface{}{
			"email": "test@example.com",
		}
		jsonBody, _ := json.Marshal(requestBody)

		req := httptest.NewRequest("POST", "/magiclink", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-api-key", testAPIKey)

		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code, "should return 400 Bad Request for missing roles")

		mockAuthUsecase.AssertNotCalled(t, "CreateMagicLink")
	})

	t.Run("Empty Roles Array", func(t *testing.T) {
		mockAuthUsecase.ExpectedCalls = nil
		mockAuthUsecase.Calls = nil
		mockAuthUsecase.On("CheckUserExists", mock.Anything, "test@example.com").Return(nil, nil)

		requestBody := requests.SupertokenPasswordlessCreateMagicLink{
			Email: "test@example.com",
			Roles: []string{},
		}
		jsonBody, _ := json.Marshal(requestBody)

		req := httptest.NewRequest("POST", "/magiclink", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-api-key", testAPIKey)

		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code, "should return 400 Bad Request for empty roles array")

		mockAuthUsecase.AssertNotCalled(t, "CreateMagicLink")
	})

	t.Run("Invalid Role", func(t *testing.T) {
		mockAuthUsecase.ExpectedCalls = nil
		mockAuthUsecase.Calls = nil
		mockAuthUsecase.On("CheckUserExists", mock.Anything, "test@example.com").Return(nil, nil)

		requestBody := requests.SupertokenPasswordlessCreateMagicLink{
			Email: "test@example.com",
			Roles: []string{"InvalidRole"},
		}
		jsonBody, _ := json.Marshal(requestBody)

		req := httptest.NewRequest("POST", "/magiclink", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-api-key", testAPIKey)

		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code, "should return 400 Bad Request for invalid role")

		mockAuthUsecase.AssertNotCalled(t, "CreateMagicLink")
	})

	t.Run("Multiple Valid Roles", func(t *testing.T) {
		mockAuthUsecase.ExpectedCalls = nil
		mockAuthUsecase.Calls = nil
		mockAuthUsecase.On("CheckUserExists", mock.Anything, "test@example.com").Return(nil, nil)

		mockAuthUsecase.On("CreateMagicLink", mock.Anything, mock.AnythingOfType("*requests.SupertokenPasswordlessCreateMagicLink")).Return(nil)

		requestBody := requests.SupertokenPasswordlessCreateMagicLink{
			Email: "test@example.com",
			Roles: []string{"Practitioner", "Researcher"},
		}
		jsonBody, _ := json.Marshal(requestBody)

		req := httptest.NewRequest("POST", "/magiclink", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-api-key", testAPIKey)

		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code, "should return 200 OK for multiple valid roles")
		mockAuthUsecase.AssertExpectations(t)
	})

	t.Run("All Valid Roles", func(t *testing.T) {
		mockAuthUsecase.ExpectedCalls = nil
		mockAuthUsecase.Calls = nil
		mockAuthUsecase.On("CheckUserExists", mock.Anything, "test@example.com").Return(nil, nil)

		mockAuthUsecase.On("CreateMagicLink", mock.Anything, mock.AnythingOfType("*requests.SupertokenPasswordlessCreateMagicLink")).Return(nil)

		requestBody := requests.SupertokenPasswordlessCreateMagicLink{
			Email: "test@example.com",
			Roles: []string{"Patient", "Practitioner", "Clinic Admin", "Researcher"},
		}
		jsonBody, _ := json.Marshal(requestBody)

		req := httptest.NewRequest("POST", "/magiclink", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-api-key", testAPIKey)

		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code, "should return 200 OK for all valid roles")
		mockAuthUsecase.AssertExpectations(t)
	})

	t.Run("Email and Roles Sanitization", func(t *testing.T) {
		mockAuthUsecase.ExpectedCalls = nil
		mockAuthUsecase.Calls = nil
		mockAuthUsecase.On("CheckUserExists", mock.Anything, "test@example.com").Return(nil, nil)

		mockAuthUsecase.On("CreateMagicLink", mock.Anything, mock.MatchedBy(func(req *requests.SupertokenPasswordlessCreateMagicLink) bool {
			// Verify that email is sanitized (lowercase, trimmed)
			if req.Email != "test@example.com" {
				return false
			}
			// Verify that roles are sanitized (trimmed)
			if len(req.Roles) != 2 {
				return false
			}
			if req.Roles[0] != "Practitioner" || req.Roles[1] != "Researcher" {
				return false
			}
			return true
		})).Return(nil)

		// Send request with unsanitized data
		requestBody := map[string]interface{}{
			"email": "  TEST@EXAMPLE.COM  ",
			"roles": []string{"  Practitioner  ", "  Researcher  "},
		}
		jsonBody, _ := json.Marshal(requestBody)

		req := httptest.NewRequest("POST", "/magiclink", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-api-key", testAPIKey)

		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code, "should return 200 OK and sanitize input data")
		mockAuthUsecase.AssertExpectations(t)
	})

	t.Run("MagicLink for Existing User without Roles", func(t *testing.T) {
		mockAuthUsecase.ExpectedCalls = nil
		mockAuthUsecase.Calls = nil

		mockAuthUsecase.On("CheckUserExists", mock.Anything, "existing@example.com").Return(makeUserExistsOutput("existing@example.com"), nil)
		mockAuthUsecase.On("CreateMagicLink", mock.Anything, mock.AnythingOfType("*requests.SupertokenPasswordlessCreateMagicLink")).Return(nil)

		requestBody := requests.SupertokenPasswordlessCreateMagicLink{
			Email: "existing@example.com",
			// No roles provided for existing user
		}
		jsonBody, _ := json.Marshal(requestBody)

		req := httptest.NewRequest("POST", "/magiclink", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-api-key", testAPIKey)

		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code, "should return 200 OK for existing user without roles")
		mockAuthUsecase.AssertExpectations(t)
	})

	t.Run("MagicLink for New User without Roles", func(t *testing.T) {
		mockAuthUsecase.ExpectedCalls = nil
		mockAuthUsecase.Calls = nil

		mockAuthUsecase.On("CheckUserExists", mock.Anything, "newuser@example.com").Return(nil, nil)

		requestBody := requests.SupertokenPasswordlessCreateMagicLink{
			Email: "newuser@example.com",
			// No roles provided for new user - should fail
		}
		jsonBody, _ := json.Marshal(requestBody)

		req := httptest.NewRequest("POST", "/magiclink", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-api-key", testAPIKey)

		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code, "should return 400 Bad Request for new user without roles")
		mockAuthUsecase.AssertNotCalled(t, "CreateMagicLink")
	})

	t.Run("MagicLink for New User with Empty Roles", func(t *testing.T) {
		mockAuthUsecase.ExpectedCalls = nil
		mockAuthUsecase.Calls = nil

		mockAuthUsecase.On("CheckUserExists", mock.Anything, "newuser@example.com").Return(nil, nil)

		requestBody := requests.SupertokenPasswordlessCreateMagicLink{
			Email: "newuser@example.com",
			Roles: []string{}, // Empty roles array
		}
		jsonBody, _ := json.Marshal(requestBody)

		req := httptest.NewRequest("POST", "/magiclink", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-api-key", testAPIKey)

		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code, "should return 400 Bad Request for new user with empty roles")
		mockAuthUsecase.AssertNotCalled(t, "CreateMagicLink")
	})

	t.Run("MagicLink for Existing User with Roles", func(t *testing.T) {
		mockAuthUsecase.ExpectedCalls = nil
		mockAuthUsecase.Calls = nil

		mockAuthUsecase.On("CheckUserExists", mock.Anything, "existing@example.com").Return(makeUserExistsOutput("existing@example.com"), nil)
		mockAuthUsecase.On("CreateMagicLink", mock.Anything, mock.AnythingOfType("*requests.SupertokenPasswordlessCreateMagicLink")).Return(nil)

		requestBody := requests.SupertokenPasswordlessCreateMagicLink{
			Email: "existing@example.com",
			Roles: []string{"Practitioner", "Researcher"}, // Roles provided for existing user
		}
		jsonBody, _ := json.Marshal(requestBody)

		req := httptest.NewRequest("POST", "/magiclink", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-api-key", testAPIKey)

		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code, "should return 200 OK for existing user with roles")
		mockAuthUsecase.AssertExpectations(t)
	})
}
