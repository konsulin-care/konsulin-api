package controllers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/fhir_dto"
	"konsulin-service/internal/pkg/utils"

	"go.uber.org/zap"
)

type ScheduleController struct {
	Usecase contracts.SlotUsecaseIface
	Log     *zap.Logger
}

func NewScheduleController(usecase contracts.SlotUsecaseIface, log *zap.Logger) *ScheduleController {
	return &ScheduleController{
		Usecase: usecase,
		Log:     log,
	}
}

type SetUnavailableRequest struct {
	PractitionerRoleIDs []string `json:"practitionerRoleIds"`
	Date                string   `json:"date,omitempty"` // YYYY-MM-DD when allDay=true
	From                string   `json:"from,omitempty"` // RFC3339 with TZ when allDay=false
	To                  string   `json:"to,omitempty"`   // RFC3339 with TZ when allDay=false
	AllDay              bool     `json:"allDay,omitempty"`
	SetStatus           string   `json:"setStatus,omitempty"`
	Reason              string   `json:"reason"`
}

func (r *SetUnavailableRequest) validate() error {
	if len(r.PractitionerRoleIDs) == 0 {
		return fmt.Errorf("practitionerRoleIds must be non-empty")
	}
	if r.Reason == "" {
		return fmt.Errorf("reason is required")
	}
	if r.AllDay {
		if r.Date == "" {
			return fmt.Errorf("date is required when allDay=true")
		}
		if _, err := time.Parse("2006-01-02", r.Date); err != nil {
			return fmt.Errorf("date must be YYYY-MM-DD")
		}
		return nil
	}
	if r.From == "" || r.To == "" {
		return fmt.Errorf("from and to are required when allDay is false or omitted")
	}
	if _, err := time.Parse(time.RFC3339, r.From); err != nil {
		return fmt.Errorf("from must be RFC3339 with timezone")
	}
	if _, err := time.Parse(time.RFC3339, r.To); err != nil {
		return fmt.Errorf("to must be RFC3339 with timezone")
	}
	return nil
}

// SetUnavailable parses request, validates input and delegates to usecase.
func (c *ScheduleController) SetUnavailable(w http.ResponseWriter, r *http.Request) {
	var req SetUnavailableRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.BuildErrorResponse(c.Log, w, exceptions.BuildNewCustomError(err, constvars.StatusBadRequest, constvars.ErrClientCannotProcessRequest, "invalid JSON"))
		return
	}
	if err := req.validate(); err != nil {
		utils.BuildErrorResponse(c.Log, w, exceptions.BuildNewCustomError(nil, constvars.StatusBadRequest, constvars.ErrClientCannotProcessRequest, err.Error()))
		return
	}

	slotStatus, err := fhir_dto.ParseSlotStatus(req.SetStatus)
	if err != nil {
		utils.BuildErrorResponse(
			c.Log, w, exceptions.BuildNewCustomError(
				err,
				constvars.StatusBadRequest,
				constvars.ErrClientCannotProcessRequest,
				"invalid setStatus value",
			))
		return
	}

	outcome, err := c.Usecase.HandleSetUnavailabilityForMultiplePractitionerRoles(r.Context(), contracts.SetUnavailabilityForMultiplePractitionerRolesInput{
		PractitionerRoleIDs: req.PractitionerRoleIDs,
		AllDay:              req.AllDay,
		AllDayDate:          req.Date,
		StartTime:           mustParseRFC3339(req.From),
		EndTime:             mustParseRFC3339(req.To),
		Reason:              req.Reason,
		SlotStatus:          slotStatus,
	})

	// this indicates an unknown error and should maintain
	// default error response. when the outcome is not nil, it means
	// the error can still be adjusted to include the outcome.
	if err != nil && outcome == nil {
		utils.BuildErrorResponse(c.Log, w, err)
		return
	}

	code := http.StatusOK
	if outcome.Created {
		code = http.StatusCreated
	}

	if len(outcome.Conflicts) > 0 {
		code = http.StatusConflict
	}

	payload := map[string]any{
		"createdSlots":             outcome.CreatedSlots,
		"conflicts":                outcome.Conflicts,
		"updatedPractitionerRoles": outcome.UpdatedPractitionerIDs,
	}
	msg := "OK"
	if code == http.StatusCreated {
		msg = "Created"
	}

	if code == http.StatusConflict {
		msg = "Conflict"
	}

	utils.BuildSuccessResponse(w, code, msg, payload)
}

func mustParseRFC3339(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, _ := time.Parse(time.RFC3339, s)
	return t
}
