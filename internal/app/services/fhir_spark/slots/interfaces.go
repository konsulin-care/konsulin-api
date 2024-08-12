package slots

import (
	"context"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/dto/responses"
)

type SlotUsecase interface{}

type SlotRepository interface{}

type SlotFhirClient interface {
	FindSlotByScheduleID(ctx context.Context, scheduleID string) ([]responses.Slot, error)
	FindSlotByScheduleIDAndStatus(ctx context.Context, scheduleID, status string) ([]responses.Slot, error)
	CreateSlot(ctx context.Context, request *requests.Slot) (*responses.Slot, error)
}
