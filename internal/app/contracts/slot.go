package contracts

import (
	"context"
	"fmt"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/fhir_dto"
	"net/url"
	"strings"
	"time"
)

type SlotFhirClient interface {
	FindSlotByScheduleID(ctx context.Context, scheduleID string) ([]fhir_dto.Slot, error)
	FindSlotByScheduleAndTimeRange(ctx context.Context, scheduleID string, startTime time.Time, endTime time.Time) ([]fhir_dto.Slot, error)
	FindSlotByScheduleIDAndStatus(ctx context.Context, scheduleID, status string) ([]fhir_dto.Slot, error)
	CreateSlot(ctx context.Context, request *fhir_dto.Slot) (*fhir_dto.Slot, error)
	// New generic finder with search params (start/end/status). Caller provides comparator in values.
	FindSlotsByScheduleWithQuery(ctx context.Context, scheduleID string, params SlotSearchParams) ([]fhir_dto.Slot, error)
	// Post transaction bundle (creates/deletes)
	PostTransactionBundle(ctx context.Context, bundle map[string]any) (*fhir_dto.FHIRBundle, error)
}

// SlotSearchParams represents supported Slot search parameters.
// Values should include any FHIR comparators (e.g., "lt2025-12-01T00:00:00+07:00").
type SlotSearchParams struct {
	Start string

	// End query for "end" is not actually supported
	// by the underlying BLAZE, but can actually still
	// be achieved by using "start" upper bounded by the end time
	End    string
	Status fhir_dto.SlotStatus
}

func (p SlotSearchParams) ToQueryString() string {
	var sb strings.Builder
	if p.Start != "" {
		sb.WriteString("&start=")
		sb.WriteString(url.QueryEscape(p.Start))
	}
	if p.End != "" {
		sb.WriteString("&start=")
		sb.WriteString(url.QueryEscape(p.End))
	}
	if p.Status != "" {
		sb.WriteString("&status=")
		sb.WriteString(url.QueryEscape(string(p.Status)))
	}
	return sb.String()
}

type SetUnavailabilityForMultiplePractitionerRolesInput struct {
	PractitionerRoleIDs []string
	AllDay              bool
	AllDayDate          string
	StartTime           time.Time
	EndTime             time.Time
	Reason              string
	SlotStatus          fhir_dto.SlotStatus
}

func (input *SetUnavailabilityForMultiplePractitionerRolesInput) Validate() error {
	if len(input.PractitionerRoleIDs) == 0 {
		return exceptions.BuildNewCustomError(nil, constvars.StatusBadRequest, constvars.ErrClientCannotProcessRequest, "practitioner role ids are required")
	}
	if input.AllDay && input.AllDayDate == "" {
		return exceptions.BuildNewCustomError(nil, constvars.StatusBadRequest, constvars.ErrClientCannotProcessRequest, "all day date is required when all day is true")
	}

	if !input.AllDay {
		if input.StartTime.IsZero() || input.EndTime.IsZero() {
			return exceptions.BuildNewCustomError(nil, constvars.StatusBadRequest, constvars.ErrClientCannotProcessRequest, "start and end time are required")
		}

		if input.EndTime.Before(input.StartTime) {
			return exceptions.BuildNewCustomError(nil, constvars.StatusBadRequest, constvars.ErrClientCannotProcessRequest, "end time must be after start time")
		}
	}

	if input.Reason == "" {
		return exceptions.BuildNewCustomError(nil, constvars.StatusBadRequest, constvars.ErrClientCannotProcessRequest, "reason is required")
	}

	// only allowing certain slot statuses for unavailability
	switch input.SlotStatus {
	default:
		return fmt.Errorf("slot status %s is not supported", input.SlotStatus)
	case fhir_dto.SlotStatusBusy, fhir_dto.SlotStatusBusyUnavailable, fhir_dto.SlotStatusBusyTentative:
		return nil
	}
}

type CreatedSlotItem struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

type ConflictingSlotItem struct {
	PractitionerRoleID string `json:"practitionerRoleId"`
	SlotID             string `json:"slotId"`
	Start              string `json:"start"`
	End                string `json:"end"`
	Status             string `json:"status"`
}

type SetUnavailableOutcome struct {
	Created                bool
	CreatedSlots           []CreatedSlotItem
	UpdatedPractitionerIDs []string
	Conflicts              []ConflictingSlotItem
}

type SlotUsecaseIface interface {
	HandleAutomatedSlotGeneration(ctx context.Context, practitionerRole fhir_dto.PractitionerRole)
	HandleSetUnavailabilityForMultiplePractitionerRoles(ctx context.Context, input SetUnavailabilityForMultiplePractitionerRolesInput) (*SetUnavailableOutcome, error)
}
