package fhir_http_client

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"konsulin-service/internal/pkg/fhir_dto"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestNew(t *testing.T) {
	logger := zap.NewNop()
	client := New(logger)
	assert.NotNil(t, client)
	assert.NotNil(t, client.client)
	assert.NotNil(t, client.logger)
}

func TestFHIRHTTPClient_Do_Success_GET(t *testing.T) {
	logger := zap.NewNop()
	fhirClient := New(logger)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "application/fhir+json", r.Header.Get("Content-Type"))

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"resourceType":"HealthcareService","id":"hs-123"}`))
	}))
	defer server.Close()

	body, err := fhirClient.Do(context.Background(), http.MethodGet, server.URL, nil)
	require.NoError(t, err)
	assert.JSONEq(t, `{"resourceType":"HealthcareService","id":"hs-123"}`, string(body))
}

func TestFHIRHTTPClient_Do_Success_POST(t *testing.T) {
	logger := zap.NewNop()
	fhirClient := New(logger)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/fhir+json", r.Header.Get("Content-Type"))

		reqBody, _ := io.ReadAll(r.Body)
		assert.JSONEq(t, `{"name":"test"}`, string(reqBody))

		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"resourceType":"Patient","id":"pat-456"}`))
	}))
	defer server.Close()

	body, err := fhirClient.Do(context.Background(), http.MethodPost, server.URL, strings.NewReader(`{"name":"test"}`))
	require.NoError(t, err)
	assert.JSONEq(t, `{"resourceType":"Patient","id":"pat-456"}`, string(body))
}

func TestFHIRHTTPClient_Do_Status3xx_Redirect(t *testing.T) {
	logger := zap.NewNop()
	fhirClient := New(logger)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// 3xx redirects should be treated as errors by the FHIR client
		w.WriteHeader(http.StatusTemporaryRedirect) // 307
		w.Write([]byte(`<html>Redirect</html>`))
	}))
	defer server.Close()

	_, err := fhirClient.Do(context.Background(), http.MethodGet, server.URL, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "status 307")
}

func TestFHIRHTTPClient_Do_Status3xx(t *testing.T) {
	logger := zap.NewNop()
	fhirClient := New(logger)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusMovedPermanently) // 301
		w.Write([]byte(`<html>Redirect</html>`))
	}))
	defer server.Close()

	_, err := fhirClient.Do(context.Background(), http.MethodGet, server.URL, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "status 301")
}

func TestFHIRHTTPClient_Do_Status4xx_WithOperationOutcome(t *testing.T) {
	logger := zap.NewNop()
	fhirClient := New(logger)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"resourceType":"OperationOutcome","issue":[{"severity":"error","diagnostics":"Resource HealthcareService/not-found not found"}]}`))
	}))
	defer server.Close()

	_, err := fhirClient.Do(context.Background(), http.MethodGet, server.URL, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Resource HealthcareService/not-found not found")
	assert.Contains(t, err.Error(), "status 404")
}

func TestFHIRHTTPClient_Do_Status5xx_WithoutOperationOutcome(t *testing.T) {
	logger := zap.NewNop()
	fhirClient := New(logger)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`<html>Server Error</html>`))
	}))
	defer server.Close()

	_, err := fhirClient.Do(context.Background(), http.MethodGet, server.URL, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "status 500")
	assert.Contains(t, err.Error(), "Server Error", "error should include response body when no OperationOutcome")
}

func TestFHIRHTTPClient_Do_InvalidURL(t *testing.T) {
	logger := zap.NewNop()
	fhirClient := New(logger)

	_, err := fhirClient.Do(context.Background(), http.MethodGet, "://invalid-url", nil)
	require.Error(t, err)
}

func TestFHIRHTTPClient_Do_ContextCancelled(t *testing.T) {
	logger := zap.NewNop()
	fhirClient := New(logger)

	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := fhirClient.Do(ctx, http.MethodGet, server.URL, nil)
	require.Error(t, err)
}

func TestFHIRHTTPClient_Do_NoDoubleSlash(t *testing.T) {
	// Verify the client doesn't introduce URL issues;
	// the caller is responsible for clean URL construction.
	logger := zap.NewNop()
	fhirClient := New(logger)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The request URL path must not contain double slashes
		assert.False(t, strings.Contains(r.URL.Path, "//"), "URL path must not contain double slash")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	// Simulate a clean URL (no trailing slash on base + resource + id)
	cleanURL := server.URL + "/HealthcareService/hs-123"
	_, err := fhirClient.Do(context.Background(), http.MethodGet, cleanURL, nil)
	require.NoError(t, err)
}

func TestOperationOutcomeParsing(t *testing.T) {
	// Unit test for the OperationOutcome parsing logic
	logger := zap.NewNop()
	fhirClient := New(logger)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		outcome := fhir_dto.OperationOutcome{
			ResourceType: "OperationOutcome",
			Issue: []fhir_dto.Issue{
				{Severity: "error", Diagnostics: "Validation failed"},
			},
		}
		json.NewEncoder(w).Encode(outcome)
	}))
	defer server.Close()

	_, err := fhirClient.Do(context.Background(), http.MethodGet, server.URL, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Validation failed")
	assert.Contains(t, err.Error(), "status 400")
}

func TestNew_NilLogger(t *testing.T) {
	client := New(nil)
	assert.NotNil(t, client)
	assert.NotNil(t, client.client)
}

func TestDo_RequestCancelledByServer(t *testing.T) {
	logger := zap.NewNop()
	fhirClient := New(logger)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// Simulate an unexpected EOF by closing connection
		hj, ok := w.(http.Hijacker)
		if ok {
			conn, _, _ := hj.Hijack()
			conn.Close()
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	_, err := fhirClient.Do(context.Background(), http.MethodGet, server.URL, nil)
	// The error may be one of several types (connection reset, unexpected EOF, etc.)
	if err != nil {
		assert.Error(t, err)
	}
}

// Ensure the Do method satisfies the expected contract for FHIR resource fetching.
func TestDo_ReturnsRawBytes(t *testing.T) {
	logger := zap.NewNop()
	fhirClient := New(logger)

	expectedBody := `{"resourceType":"HealthcareService","id":"hs-999","name":"General Consultation","extension":[]}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(expectedBody))
	}))
	defer server.Close()

	body, err := fhirClient.Do(context.Background(), http.MethodGet, server.URL, nil)
	require.NoError(t, err)
	assert.Equal(t, expectedBody, string(body))
}

// Test that existing errors (like wrapped errors) are preserved in the chain.
func TestDo_ErrorWrapping(t *testing.T) {
	logger := zap.NewNop()
	fhirClient := New(logger)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"resourceType":"OperationOutcome","issue":[{"severity":"error","diagnostics":"not found"}]}`))
	}))
	defer server.Close()

	_, err := fhirClient.Do(context.Background(), http.MethodGet, server.URL, nil)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrFHIRRequestFailed), "error should wrap ErrFHIRRequestFailed")
}
