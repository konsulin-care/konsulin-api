package contracts

import (
	"context"
	"konsulin-service/internal/pkg/fhir_dto"
)

type SlotFhirClient interface {
	FindSlotByScheduleID(ctx context.Context, scheduleID string) ([]fhir_dto.Slot, error)
	FindSlotByScheduleIDAndStatus(ctx context.Context, scheduleID, status string) ([]fhir_dto.Slot, error)
	CreateSlot(ctx context.Context, request *fhir_dto.Slot) (*fhir_dto.Slot, error)
}
