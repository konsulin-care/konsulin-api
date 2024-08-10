package slots

import (
	"context"
	"konsulin-service/internal/pkg/dto/responses"
	"time"
)

type SlotUsecase interface{}

type SlotRepository interface{}

type SlotFhirClient interface {
	FindSlotByScheduleID(ctx context.Context, scheduleID string) ([]responses.Slot, error)
	CreateSlotOnDemand(ctx context.Context, clinicianID, date, startTime string, endTime time.Time) (*responses.Slot, error)
}
