package plan_definitions

import (
	"context"
	"sync"

	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/app/services/fhir_spark/base"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/fhir_dto"
	"konsulin-service/internal/pkg/fhir_http_client"

	"go.uber.org/zap"
)

var (
	planDefinitionFhirClientInstance contracts.PlanDefinitionFhirClient
	oncePlanDefinitionFhirClient     sync.Once
)

type planDefinitionFhirClient struct {
	*base.ResourceClient
}

// NewPlanDefinitionFhirClient returns a singleton PlanDefinitionFhirClient bound
// to the given FHIR base URL. All HTTP traffic goes through FHIRHTTPClient.Do.
func NewPlanDefinitionFhirClient(baseUrl string, logger *zap.Logger) contracts.PlanDefinitionFhirClient {
	oncePlanDefinitionFhirClient.Do(func() {
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
