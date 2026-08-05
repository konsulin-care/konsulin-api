package fhir_http_client

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestDo_NotFoundReturnsTypedStatusError(t *testing.T) {
	logger := zap.NewNop()
	fhirClient := New(logger)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/fhir+json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"resourceType":"OperationOutcome","issue":[{"severity":"error","code":"not-found","diagnostics":"Patient not found"}]}`))
	}))
	defer server.Close()

	_, err := fhirClient.Do(context.Background(), http.MethodGet, server.URL, nil)
	require.Error(t, err)

	var httpErr *FHIRHTTPError
	assert.True(t, errors.As(err, &httpErr), "expected *FHIRHTTPError, got %T", err)
	assert.Equal(t, http.StatusNotFound, httpErr.StatusCode)
	assert.True(t, errors.Is(err, ErrFHIRRequestFailed))
	assert.Contains(t, err.Error(), "not found")
}
