package plan_definitions

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"konsulin-service/internal/app/services/fhir_spark/base"
	"konsulin-service/internal/pkg/constvars"
	"strings"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func newTestClient(baseURL string, logger *zap.Logger) *planDefinitionFhirClient {
	// Mirror the production APP_FHIR_BASE_URL shape (trailing slash).
	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}
	return &planDefinitionFhirClient{
		ResourceClient: base.New(baseURL, constvars.ResourcePlanDefinition, logger),
	}
}

func TestNewPlanDefinitionFhirClient(t *testing.T) {
	logger := zap.NewNop()
	client := NewPlanDefinitionFhirClient("http://fhir.example.com", logger)
	require.NotNil(t, client)

	_, ok := client.(*planDefinitionFhirClient)
	assert.True(t, ok, "expected *planDefinitionFhirClient")
}

func TestPlanDefinitionFhirClient_FindPlanDefinitionByID_Success(t *testing.T) {
	logger := zap.NewNop()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/PlanDefinition/batch-1", r.URL.Path)
		w.Header().Set("Content-Type", "application/fhir+json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"resourceType":"PlanDefinition","id":"batch-1","status":"active","name":"ResearchBatch"}`))
	}))
	defer server.Close()

	client := newTestClient(server.URL, logger)
	plan, err := client.FindPlanDefinitionByID(context.Background(), "batch-1")
	require.NoError(t, err)
	require.NotNil(t, plan)
	assert.Equal(t, "batch-1", plan.ID)
	assert.Equal(t, "active", plan.Status)
	assert.Equal(t, "ResearchBatch", plan.Name)
}

func TestPlanDefinitionFhirClient_FindPlanDefinitionByID_NotFound(t *testing.T) {
	logger := zap.NewNop()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/fhir+json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"resourceType":"OperationOutcome","issue":[{"severity":"error","code":"not-found","diagnostics":"PlanDefinition batch-missing not found"}]}`))
	}))
	defer server.Close()

	client := newTestClient(server.URL, logger)
	_, err := client.FindPlanDefinitionByID(context.Background(), "batch-missing")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "batch-missing")
}
