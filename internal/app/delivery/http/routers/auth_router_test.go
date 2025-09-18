package routers

import (
	"bytes"
	"context"
	"encoding/json"
	"konsulin-service/internal/app/config"
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
	"go.uber.org/zap"
)

type MockAuthUsecase struct {
	mock.Mock
}

func (m *MockAuthUsecase) InitializeSupertoken() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockAuthUsecase) CreateMagicLink(ctx context.Context, request *requests.SupertokenPasswordlessCreateMagicLink) error {
	args := m.Called(ctx, request)
	return args.Error(0)
}

func (m *MockAuthUsecase) CreateAnonymousSession(ctx context.Context) (string, error) {
	args := m.Called(ctx)
	return args.String(0), args.Error(1)
}

func (m *MockAuthUsecase) LogoutUser(ctx context.Context, sessionData string) error {
	args := m.Called(ctx, sessionData)
	return args.Error(0)
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

	authController := controllers.NewAuthController(logger, mockAuthUsecase)

	middlewareInstance := &middlewares.Middlewares{
		Log:            logger,
		InternalConfig: internalConfig,
	}

	router := chi.NewRouter()
	attachAuthRoutes(router, middlewareInstance, authController)

	t.Run("MagicLink with Valid API Key", func(t *testing.T) {

		mockAuthUsecase.On("CreateMagicLink", mock.Anything, mock.AnythingOfType("*requests.SupertokenPasswordlessCreateMagicLink")).Return(nil)

		requestBody := requests.SupertokenPasswordlessCreateMagicLink{
			Email: "test@example.com",
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

	t.Run("MagicLink without API Key", func(t *testing.T) {

		requestBody := requests.SupertokenPasswordlessCreateMagicLink{
			Email: "test@example.com",
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

		mockAuthUsecase.On("CreateAnonymousSession", mock.Anything).Return("test-session-handle", nil)

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

	authController := controllers.NewAuthController(logger, mockAuthUsecase)

	middlewareInstance := &middlewares.Middlewares{
		Log:            logger,
		InternalConfig: internalConfig,
	}

	router := chi.NewRouter()
	attachAuthRoutes(router, middlewareInstance, authController)

	t.Run("Context Values Propagated to Controller", func(t *testing.T) {

		mockAuthUsecase.On("CreateMagicLink", mock.MatchedBy(func(ctx context.Context) bool {

			apiKeyAuth, ok := ctx.Value(middlewares.ContextAPIKeyAuth).(bool)
			if !ok || !apiKeyAuth {
				return false
			}

			roles, ok := ctx.Value("roles").([]string)
			if !ok || len(roles) != 1 || roles[0] != constvars.KonsulinRoleSuperadmin {
				return false
			}

			uid, ok := ctx.Value("uid").(string)
			if !ok || uid != "api-key-superadmin" {
				return false
			}

			return true
		}), mock.AnythingOfType("*requests.SupertokenPasswordlessCreateMagicLink")).Return(nil)

		requestBody := requests.SupertokenPasswordlessCreateMagicLink{
			Email: "test@example.com",
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

	authController := controllers.NewAuthController(logger, mockAuthUsecase)

	middlewareInstance := &middlewares.Middlewares{
		Log:            logger,
		InternalConfig: internalConfig,
	}

	router := chi.NewRouter()
	attachAuthRoutes(router, middlewareInstance, authController)

	t.Run("Invalid JSON Body", func(t *testing.T) {

		req := httptest.NewRequest("POST", "/magiclink", bytes.NewBufferString("invalid json"))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-api-key", testAPIKey)

		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code, "should return 400 Bad Request for invalid JSON")

		mockAuthUsecase.AssertNotCalled(t, "CreateMagicLink")
	})

	t.Run("Missing Email Field", func(t *testing.T) {

		requestBody := map[string]interface{}{}
		jsonBody, _ := json.Marshal(requestBody)

		req := httptest.NewRequest("POST", "/magiclink", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-api-key", testAPIKey)

		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code, "should return 400 Bad Request for missing email")

		mockAuthUsecase.AssertNotCalled(t, "CreateMagicLink")
	})
}
