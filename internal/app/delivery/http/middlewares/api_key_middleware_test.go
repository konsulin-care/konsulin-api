package middlewares

import (
	"context"
	"konsulin-service/internal/app/config"
	"konsulin-service/internal/pkg/constvars"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestRequireSuperadminAPIKey(t *testing.T) {
	logger := zap.NewNop()

	testAPIKey := "test-superadmin-api-key-12345"
	internalConfig := &config.InternalConfig{
		App: config.App{
			SuperadminAPIKey: testAPIKey,
		},
	}

	middlewares := &Middlewares{
		Log:            logger,
		InternalConfig: internalConfig,
	}

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		apiKeyAuth, ok := r.Context().Value(ContextAPIKeyAuth).(bool)
		assert.True(t, ok, "ContextAPIKeyAuth should be set")
		assert.True(t, apiKeyAuth, "ContextAPIKeyAuth should be true")

		roles, ok := r.Context().Value(keyRoles).([]string)
		assert.True(t, ok, "roles should be set in context")
		assert.Len(t, roles, 1, "should have exactly one role")
		assert.Equal(t, constvars.KonsulinRoleSuperadmin, roles[0], "role should be superadmin")

		uid, ok := r.Context().Value(keyUID).(string)
		assert.True(t, ok, "uid should be set in context")
		assert.Equal(t, "api-key-superadmin", uid, "uid should be api-key-superadmin")

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	t.Run("Valid API Key", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/v1/auth/magiclink", nil)
		req.Header.Set(HeaderAPIKey, testAPIKey)

		rr := httptest.NewRecorder()
		handler := middlewares.RequireSuperadminAPIKey(testHandler)
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code, "should return 200 OK for valid API key")
		assert.Equal(t, "success", rr.Body.String(), "should return success message")
	})

	t.Run("Missing API Key", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/v1/auth/magiclink", nil)

		rr := httptest.NewRecorder()
		handler := middlewares.RequireSuperadminAPIKey(testHandler)
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code, "should return 401 Unauthorized for missing API key")
	})

	t.Run("Invalid API Key", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/v1/auth/magiclink", nil)
		req.Header.Set(HeaderAPIKey, "invalid-api-key")

		rr := httptest.NewRecorder()
		handler := middlewares.RequireSuperadminAPIKey(testHandler)
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code, "should return 401 Unauthorized for invalid API key")
	})

	t.Run("Empty API Key", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/v1/auth/magiclink", nil)
		req.Header.Set(HeaderAPIKey, "")

		rr := httptest.NewRecorder()
		handler := middlewares.RequireSuperadminAPIKey(testHandler)
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code, "should return 401 Unauthorized for empty API key")
	})

	t.Run("Case Sensitivity", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/v1/auth/magiclink", nil)
		req.Header.Set(HeaderAPIKey, "TEST-SUPERADMIN-API-KEY-12345")

		rr := httptest.NewRecorder()
		handler := middlewares.RequireSuperadminAPIKey(testHandler)
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code, "should return 401 Unauthorized for case-mismatched API key")
	})

	t.Run("Whitespace in API Key", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/v1/auth/magiclink", nil)
		req.Header.Set(HeaderAPIKey, " "+testAPIKey+" ")

		rr := httptest.NewRecorder()
		handler := middlewares.RequireSuperadminAPIKey(testHandler)
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code, "should return 401 Unauthorized for API key with whitespace")
	})
}

func TestAPIKeyAuth_Optional(t *testing.T) {

	logger := zap.NewNop()

	testAPIKey := "test-superadmin-api-key-12345"
	internalConfig := &config.InternalConfig{
		App: config.App{
			SuperadminAPIKey: testAPIKey,
		},
	}

	middlewares := &Middlewares{
		Log:            logger,
		InternalConfig: internalConfig,
	}

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		apiKeyAuth, ok := r.Context().Value(ContextAPIKeyAuth).(bool)
		if ok {
			assert.False(t, apiKeyAuth, "ContextAPIKeyAuth should be false when no API key provided")
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	t.Run("No API Key - Should Pass", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/v1/some-endpoint", nil)

		rr := httptest.NewRecorder()
		handler := middlewares.APIKeyAuth(testHandler)
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code, "should return 200 OK when no API key provided (optional middleware)")
		assert.Equal(t, "success", rr.Body.String(), "should return success message")
	})

	t.Run("Valid API Key - Should Pass", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/v1/some-endpoint", nil)
		req.Header.Set(HeaderAPIKey, testAPIKey)

		rr := httptest.NewRecorder()
		handler := middlewares.APIKeyAuth(testHandler)
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code, "should return 200 OK for valid API key")
		assert.Equal(t, "success", rr.Body.String(), "should return success message")
	})

	t.Run("Invalid API Key - Should Fail", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/v1/some-endpoint", nil)
		req.Header.Set(HeaderAPIKey, "invalid-api-key")

		rr := httptest.NewRecorder()
		handler := middlewares.APIKeyAuth(testHandler)
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code, "should return 401 Unauthorized for invalid API key")
	})
}

func TestRequireSuperadminAPIKey_Integration(t *testing.T) {

	logger := zap.NewNop()

	testAPIKey := "test-superadmin-api-key-12345"
	internalConfig := &config.InternalConfig{
		App: config.App{
			SuperadminAPIKey: testAPIKey,
		},
	}

	middlewares := &Middlewares{
		Log:            logger,
		InternalConfig: internalConfig,
	}

	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH"}

	for _, method := range methods {
		t.Run("Method_"+method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/v1/auth/magiclink", nil)
			req.Header.Set(HeaderAPIKey, testAPIKey)

			rr := httptest.NewRecorder()
			handler := middlewares.RequireSuperadminAPIKey(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("success"))
			}))
			handler.ServeHTTP(rr, req)

			assert.Equal(t, http.StatusOK, rr.Code, "should return 200 OK for %s method with valid API key", method)
		})
	}
}

func TestRequireSuperadminAPIKey_ContextValues(t *testing.T) {

	logger := zap.NewNop()

	testAPIKey := "test-superadmin-api-key-12345"
	internalConfig := &config.InternalConfig{
		App: config.App{
			SuperadminAPIKey: testAPIKey,
		},
	}

	middlewares := &Middlewares{
		Log:            logger,
		InternalConfig: internalConfig,
	}

	t.Run("Context Values Set Correctly", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/v1/auth/magiclink", nil)
		req.Header.Set(HeaderAPIKey, testAPIKey)

		var capturedContext context.Context
		testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedContext = r.Context()
			w.WriteHeader(http.StatusOK)
		})

		rr := httptest.NewRecorder()
		handler := middlewares.RequireSuperadminAPIKey(testHandler)
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code, "should return 200 OK")

		apiKeyAuth, ok := capturedContext.Value(ContextAPIKeyAuth).(bool)
		assert.True(t, ok, "ContextAPIKeyAuth should be set")
		assert.True(t, apiKeyAuth, "ContextAPIKeyAuth should be true")

		roles, ok := capturedContext.Value(keyRoles).([]string)
		assert.True(t, ok, "roles should be set in context")
		assert.Len(t, roles, 1, "should have exactly one role")
		assert.Equal(t, constvars.KonsulinRoleSuperadmin, roles[0], "role should be superadmin")

		uid, ok := capturedContext.Value(keyUID).(string)
		assert.True(t, ok, "uid should be set in context")
		assert.Equal(t, "api-key-superadmin", uid, "uid should be api-key-superadmin")
	})
}
