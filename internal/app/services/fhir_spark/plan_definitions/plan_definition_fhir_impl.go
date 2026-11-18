package plan_definitions

import (
	"context"
	"sync"

	"github.com/konsulin-care/konsulin-api/internal/app/contracts"
	"github.com/konsulin-care/konsulin-api/internal/app/services/fhir_spark/base"
	"github.com/konsulin-care/konsulin-api/internal/pkg/constvars"
	"github.com/konsulin-care/konsulin-api/internal/pkg/fhir_dto"
	"github.com/konsulin-care/konsulin-api/internal/pkg/fhir_http_client"

	"go.uber.org/zap"
)

var (
	planDefinitionFhirClientInstance contracts.PlanDefinitionFinder
	oncePlanDefinitionFinder         sync.Once
)

type planDefinitionFhirClient struct {
	*base.ResourceClient
}

// NewPlanDefinitionFinder returns a singleton PlanDefinitionFinder bound
// to the given FHIR base URL. All HTTP traffic goes through FHIRHTTPClient.Do.
func NewPlanDefinitionFinder(baseUrl string, logger *zap.Logger) contracts.PlanDefinitionFinder {
	oncePlanDefinitionFinder.Do(func() {
		planDefinitionFhirClientInstance = &planDefinitionFhirClient{
			ResourceClient: base.New(baseUrl, constvars.ResourcePlanDefinition, logger),
		}
	})
	return planDefinitionFhirClientInstance
}

// FindPlanDefinitionByID returns the PlanDefinition resource with the given id,
// or an error when it does not exist.
func (c *planDefinitionFhirClient) FindPlanDefinitionByID(ctx context.Context, planDefinitionID string) (*fhir_dto.PlanDefinition, error) {
	return fhir_http_client.GetResource[fhir_dto.PlanDefinition](ctx, c.Log, c.Client, c.BaseUrl, planDefinitionID,
		constvars.ResourcePlanDefinition, constvars.LoggingPlanDefinitionIDKey)
}
