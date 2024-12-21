package contracts

import (
	"context"
	"konsulin-service/internal/pkg/fhir_dto"
)

type ChargeItemDefinitionFhirClient interface {
	CreateChargeItemDefinition(ctx context.Context, request *fhir_dto.ChargeItemDefinition) (*fhir_dto.ChargeItemDefinition, error)
	UpdateChargeItemDefinition(ctx context.Context, request *fhir_dto.ChargeItemDefinition) (*fhir_dto.ChargeItemDefinition, error)
	PatchChargeItemDefinition(ctx context.Context, request *fhir_dto.ChargeItemDefinition) (*fhir_dto.ChargeItemDefinition, error)
	FindChargeItemDefinitionByID(ctx context.Context, chargeItemDefinitionID string) (*fhir_dto.ChargeItemDefinition, error)
	FindChargeItemDefinitionByPractitionerRoleID(ctx context.Context, practitionerRoleID string) (*fhir_dto.ChargeItemDefinition, error)
}
