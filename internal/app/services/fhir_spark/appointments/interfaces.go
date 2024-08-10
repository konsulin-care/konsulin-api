package appointments

import (
	"context"
	"time"
)

type AppointmentUsecase interface{}

type AppointmentRepository interface{}

type AppointmentFhirClient interface {
	CheckClinicianAvailability(ctx context.Context, clinicianId string, startTime, endTime time.Time) (bool, error)
}
