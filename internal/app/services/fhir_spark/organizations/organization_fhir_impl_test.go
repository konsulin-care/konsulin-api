package organizations

import (
	"context"
	"testing"

	"konsulin-service/internal/pkg/fhir_dto"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestNewOrganizationFhirClient(t *testing.T) {
	logger := zap.NewNop()
	client := NewOrganizationFhirClient("http://fhir.example.com", logger)
	require.NotNil(t, client)

	// Verify it returns the expected concrete type
	_, ok := client.(*organizationFhirClient)
	assert.True(t, ok, "expected *organizationFhirClient")
}

func TestOrganizationFhirClient_FindOrganizationByID_Error(t *testing.T) {
	logger := zap.NewNop()
	client := NewOrganizationFhirClient("http://nonexistent.example.com", logger)

	ctx := context.Background()
	_, err := client.FindOrganizationByID(ctx, "org-1")
	assert.Error(t, err)
}

func TestOrganizationFhirClient_Update_Error(t *testing.T) {
	logger := zap.NewNop()
	client := NewOrganizationFhirClient("http://nonexistent.example.com", logger)

	ctx := context.Background()
	_, err := client.Update(ctx, fhir_dto.Organization{ID: "org-1"})
	assert.Error(t, err)
}

func TestOrganizationFhirClient_FindAll_Error(t *testing.T) {
	logger := zap.NewNop()
	client := NewOrganizationFhirClient("http://nonexistent.example.com", logger)

	ctx := context.Background()
	_, _, err := client.FindAll(ctx, "test", "paged", 1, 10)
	assert.Error(t, err)
}
