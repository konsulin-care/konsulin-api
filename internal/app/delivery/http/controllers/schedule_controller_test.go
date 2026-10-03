package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/pkg/fhir_dto"
)

type scheduleUsecaseStub struct {
	contracts.SlotUsecaseIface
	input   contracts.SetUnavailabilityForMultiplePractitionerRolesInput
	outcome *contracts.SetUnavailableOutcome
	err     error
	calls   int
}

func (s *scheduleUsecaseStub) HandleSetUnavailabilityForMultiplePractitionerRoles(_ context.Context, in contracts.SetUnavailabilityForMultiplePractitionerRolesInput) (*contracts.SetUnavailableOutcome, error) {
	s.input = in
	s.calls++
	return s.outcome, s.err
}

func TestScheduleRequestValidation(t *testing.T) {
	base := SetUnavailableRequest{PractitionerRoleIDs: []string{"role1"}, Reason: "leave", From: "2026-10-03T09:00:00+07:00", To: "2026-10-03T10:00:00+07:00"}
	for _, tc := range []struct {
		name    string
		change  func(*SetUnavailableRequest)
		message string
	}{
		{"valid window", func(*SetUnavailableRequest) {}, ""},
		{"missing roles", func(r *SetUnavailableRequest) { r.PractitionerRoleIDs = nil }, "practitionerRoleIds must be non-empty"},
		{"missing reason", func(r *SetUnavailableRequest) { r.Reason = "" }, "reason is required"},
		{"missing all-day date", func(r *SetUnavailableRequest) { r.AllDay = true }, "date is required when allDay=true"},
		{"invalid all-day date", func(r *SetUnavailableRequest) { r.AllDay = true; r.Date = "2026-02-30" }, "date must be YYYY-MM-DD"},
		{"valid all-day", func(r *SetUnavailableRequest) { r.AllDay = true; r.Date = "2026-10-03" }, ""},
		{"missing from", func(r *SetUnavailableRequest) { r.From = "" }, "from and to are required"},
		{"invalid from", func(r *SetUnavailableRequest) { r.From = "2026-10-03" }, "from must be RFC3339"},
		{"invalid to", func(r *SetUnavailableRequest) { r.To = "2026-10-03" }, "to must be RFC3339"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := base
			tc.change(&r)
			err := r.validate()
			if tc.message == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tc.message)
			}
		})
	}
}

func TestScheduleSetUnavailableHTTPContract(t *testing.T) {
	failure := errors.New("schedule storage unavailable")
	valid := `{"practitionerRoleIds":["role1"],"reason":"leave","allDay":true,"date":"2026-10-03","setStatus":"busy-unavailable"}`
	for _, tc := range []struct {
		name, body    string
		outcome       *contracts.SetUnavailableOutcome
		err           error
		status, calls int
		message       string
	}{
		{"invalid JSON", `{`, nil, nil, 400, 0, ""},
		{"invalid request", `{}`, nil, nil, 400, 0, ""},
		{"invalid status", strings.Replace(valid, "busy-unavailable", "invalid", 1), nil, nil, 400, 0, ""},
		{"service error", valid, nil, failure, 500, 1, ""},
		{"updated", valid, &contracts.SetUnavailableOutcome{UpdatedPractitionerIDs: []string{"pr1"}}, nil, 200, 1, "OK"},
		{"created", valid, &contracts.SetUnavailableOutcome{Created: true, CreatedSlots: []contracts.CreatedSlotItem{{ID: "slot1", Status: "busy-unavailable"}}}, nil, 201, 1, "Created"},
		{"conflict", valid, &contracts.SetUnavailableOutcome{Created: true, Conflicts: []contracts.ConflictingSlotItem{{SlotID: "slot1"}}}, failure, 409, 1, "Conflict"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			uc := &scheduleUsecaseStub{outcome: tc.outcome, err: tc.err}
			ctrl := NewScheduleController(uc, zap.NewNop())
			w := httptest.NewRecorder()
			r := httptest.NewRequest("POST", "/schedule/unavailable", strings.NewReader(tc.body))
			ctrl.SetUnavailable(w, r)
			require.Equal(t, tc.status, w.Code, w.Body.String())
			require.Equal(t, tc.calls, uc.calls)
			if tc.calls > 0 {
				require.Equal(t, []string{"role1"}, uc.input.PractitionerRoleIDs)
				require.Equal(t, "2026-10-03", uc.input.AllDayDate)
				require.True(t, uc.input.AllDay)
				require.Equal(t, "leave", uc.input.Reason)
				require.Equal(t, fhir_dto.SlotStatusBusyUnavailable, uc.input.SlotStatus)
				require.True(t, uc.input.StartTime.IsZero())
				require.True(t, uc.input.EndTime.IsZero())
			}
			if tc.message != "" {
				var response struct {
					Success bool
					Message string
					Data    map[string]json.RawMessage
				}
				require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
				require.True(t, response.Success)
				require.Equal(t, tc.message, response.Message)
				require.Contains(t, response.Data, "createdSlots")
				require.Contains(t, response.Data, "conflicts")
				require.Contains(t, response.Data, "updatedPractitionerRoles")
			}
		})
	}
	parsed := mustParseRFC3339("2026-10-03T09:00:00+07:00")
	require.Equal(t, time.Date(2026, 10, 3, 2, 0, 0, 0, time.UTC), parsed.UTC())
}
