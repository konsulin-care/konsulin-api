package controllers

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/pkg/constvars"
)

type organizationUsecaseStub struct {
	contracts.OrganizationUsecase
	input contracts.RegisterPractitionerRoleInput
	err   error
	calls int
}

func (s *organizationUsecaseStub) RegisterPractitionerRoleAndSchedule(_ context.Context, input contracts.RegisterPractitionerRoleInput) (*contracts.RegisterPractitionerRoleOutput, error) {
	s.calls++
	s.input = input
	if s.err != nil {
		return nil, s.err
	}
	return &contracts.RegisterPractitionerRoleOutput{PractitionerID: "pr1", PractitionerRoleID: "role1", ScheduleID: "schedule1"}, nil
}

func TestOrganizationRegistrationHTTPContract(t *testing.T) {
	for _, tc := range []struct {
		name, requestID, org, body string
		err                        error
		status, calls              int
	}{
		{"created", "request", "org1", `{"email":"test@example.invalid"}`, nil, 201, 1},
		{"missing request ID", "", "org1", `{"email":"test@example.invalid"}`, nil, 403, 0},
		{"missing organization", "request", " ", `{"email":"test@example.invalid"}`, nil, 400, 0},
		{"malformed JSON", "request", "org1", `{`, nil, 400, 0},
		{"missing email", "request", "org1", `{}`, nil, 400, 0},
		{"blank email", "request", "org1", `{"email":" "}`, nil, 400, 0},
		{"invalid email", "request", "org1", `{"email":"invalid"}`, nil, 400, 0},
		{"service failure", "request", "org1", `{"email":"test@example.invalid"}`, errors.New("storage failure"), 500, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			uc := &organizationUsecaseStub{err: tc.err}
			ctrl := &OrganizationController{Usecase: uc, Log: zap.NewNop()}
			r := httptest.NewRequest("POST", "/organizations/role", strings.NewReader(tc.body))
			ctx := context.WithValue(r.Context(), constvars.CONTEXT_REQUEST_ID_KEY, tc.requestID)
			route := chi.NewRouteContext()
			route.URLParams.Add("organizationId", tc.org)
			r = r.WithContext(context.WithValue(ctx, chi.RouteCtxKey, route))
			w := httptest.NewRecorder()
			ctrl.RegisterPractitionerRole(w, r)
			require.Equal(t, tc.status, w.Code, w.Body.String())
			require.Equal(t, tc.calls, uc.calls)
			require.Equal(t, "application/json", w.Header().Get("Content-Type"))
			if tc.calls == 1 {
				require.Equal(t, contracts.RegisterPractitionerRoleInput{OrganizationID: "org1", Email: "test@example.invalid"}, uc.input)
			}
			if tc.status == 201 {
				require.JSONEq(t, `{"success":true,"message":"Created","data":{"practitionerId":"pr1","practitionerRoleId":"role1","scheduleId":"schedule1"}}`, w.Body.String())
			}
		})
	}
}

func TestOrganizationAndRoleConstructors(t *testing.T) {
	uc := &organizationUsecaseStub{}
	first := NewOrganizationController(zap.NewNop(), uc)
	require.Same(t, first, NewOrganizationController(zap.NewNop(), nil))
	require.Same(t, uc, first.Usecase)
	role := NewRoleController(nil)
	require.NotNil(t, role)
	require.Nil(t, role.RoleUsecase)
}
