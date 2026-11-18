package contracts

import (
	"context"

	"konsulin-service/internal/pkg/fhir_dto"
)

// PlanDefinitionFinder reads PlanDefinition resources from the FHIR server.
// Referral Communication batches reference a PlanDefinition; the middleware uses
// this client to verify the referenced batch actually exists before allowing a
// referral write to be proxied to Blaze.
type PlanDefinitionFinder interface {
	// FindPlanDefinitionByID returns the PlanDefinition with the given id, or an
	// error if it does not exist or the FHIR server could not be reached.
	FindPlanDefinitionByID(ctx context.Context, planDefinitionID string) (*fhir_dto.PlanDefinition, error)
}
