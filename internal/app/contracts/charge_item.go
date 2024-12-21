package contracts

import (
	"context"
	"konsulin-service/internal/pkg/fhir_dto"
)

type ChargeItemFhirClient interface {
	CreateChargeItem(ctx context.Context, request *fhir_dto.ChargeItem) (*fhir_dto.ChargeItem, error)
	UpdateChargeItem(ctx context.Context, request *fhir_dto.ChargeItem) (*fhir_dto.ChargeItem, error)
	PatchChargeItem(ctx context.Context, request *fhir_dto.ChargeItem) (*fhir_dto.ChargeItem, error)
	FindChargeItemByID(ctx context.Context, patientID string) (*fhir_dto.ChargeItem, error)
}
