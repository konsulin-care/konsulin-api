package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/pkg/constvars"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/supertokens/supertokens-golang/recipe/session"
	"github.com/supertokens/supertokens-golang/recipe/session/sessmodels"
	"github.com/supertokens/supertokens-golang/recipe/userroles"
	"github.com/supertokens/supertokens-golang/recipe/userroles/userrolesmodels"
	"github.com/supertokens/supertokens-golang/supertokens"
	"go.uber.org/zap"
)

func TestGetFhirResourceIDForUser(t *testing.T) {
	lookupErr := errors.New("FHIR unavailable")
	for _, tc := range []struct {
		name      string
		roles     []string
		resources contracts.InitializeNewUserFHIRResourcesOutput
		lookupErr error
		want      string
		wantErr   bool
	}{
		{name: "practitioner", roles: []string{constvars.KonsulinRolePractitioner}, resources: contracts.InitializeNewUserFHIRResourcesOutput{PractitionerID: "p", PatientID: "u"}, want: "Practitioner/p"},
		{name: "clinic admin", roles: []string{constvars.KonsulinRoleClinicAdmin}, resources: contracts.InitializeNewUserFHIRResourcesOutput{PractitionerID: "p"}, want: "Practitioner/p"},
		{name: "researcher", roles: []string{constvars.KonsulinRoleResearcher}, resources: contracts.InitializeNewUserFHIRResourcesOutput{PractitionerID: "p"}, want: "Practitioner/p"},
		{name: "practitioner wins over patient regardless of order", roles: []string{constvars.KonsulinRolePatient, constvars.KonsulinRolePractitioner}, resources: contracts.InitializeNewUserFHIRResourcesOutput{PractitionerID: "p", PatientID: "u"}, want: "Practitioner/p"},
		{name: "patient role wins over fallback practitioner", roles: []string{constvars.KonsulinRolePatient}, resources: contracts.InitializeNewUserFHIRResourcesOutput{PractitionerID: "p", PatientID: "u"}, want: "Patient/u"},
		{name: "missing practitioner falls back to patient", roles: []string{constvars.KonsulinRolePractitioner, constvars.KonsulinRolePatient}, resources: contracts.InitializeNewUserFHIRResourcesOutput{PatientID: "u"}, want: "Patient/u"},
		{name: "unrecognized role practitioner fallback", roles: []string{"unknown"}, resources: contracts.InitializeNewUserFHIRResourcesOutput{PractitionerID: "p"}, want: "Practitioner/p"},
		{name: "no roles patient fallback", resources: contracts.InitializeNewUserFHIRResourcesOutput{PatientID: "u"}, want: "Patient/u"},
		{name: "no matching resources", roles: []string{constvars.KonsulinRoleSuperadmin}, wantErr: true},
		{name: "lookup error", lookupErr: lookupErr, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			initializer := new(MockUserFHIRInitializer)
			initializer.On("LookupUserFHIRResourceIDs", mock.MatchedBy(func(ctx context.Context) bool {
				deadline, ok := ctx.Deadline()
				return ok && time.Until(deadline) > 0 && time.Until(deadline) <= 2*time.Second
			}), &contracts.LookupUserFHIRResourceIDsInput{SuperTokenUserID: "user"}).Return(&tc.resources, tc.lookupErr).Once()
			uc := &authUsecase{UserFHIRInitializer: initializer, Log: zap.NewNop()}
			got, err := uc.getFhirResourceIdForUser(context.Background(), "user", tc.roles)
			require.Equal(t, tc.want, got)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			if tc.lookupErr != nil {
				require.ErrorIs(t, err, tc.lookupErr)
			}
			initializer.AssertExpectations(t)
		})
	}
}

// rolesStub holds what the overridden SuperTokens GetRolesForUser returns, and the
// arguments it was last called with.
type rolesStub struct {
	response       userrolesmodels.GetRolesForUserResponse
	err            error
	userID, tenant string
}

func initRolesStub(t *testing.T) *rolesStub {
	t.Helper()
	supertokens.ResetForTest()
	t.Cleanup(supertokens.ResetForTest)
	stub := &rolesStub{}
	telemetry := false
	require.NoError(t, supertokens.Init(supertokens.TypeInput{
		Supertokens: &supertokens.ConnectionInfo{ConnectionURI: "http://localhost:9999"},
		AppInfo:     supertokens.AppInfo{AppName: "payload-test", APIDomain: "http://localhost:8080", WebsiteDomain: "http://localhost:3000"},
		Telemetry:   &telemetry,
		RecipeList: []supertokens.Recipe{session.Init(nil), userroles.Init(&userrolesmodels.TypeInput{Override: &userrolesmodels.OverrideStruct{Functions: func(original userrolesmodels.RecipeInterface) userrolesmodels.RecipeInterface {
			*original.GetRolesForUser = func(userID, tenant string, _ supertokens.UserContext) (userrolesmodels.GetRolesForUserResponse, error) {
				stub.userID, stub.tenant = userID, tenant
				return stub.response, stub.err
			}
			return original
		}}})},
	}))
	return stub
}

func rolesOK(roles ...string) *struct{ Roles []string } {
	return &struct{ Roles []string }{Roles: roles}
}

type accessTokenCase struct {
	name      string
	roles     *struct{ Roles []string }
	rolesErr  error
	lookup    *contracts.InitializeNewUserFHIRResourcesOutput // nil when no FHIR lookup is expected
	lookupErr error
	wantRoles []interface{}
	wantID    string
}

func TestBuildAccessTokenPayload(t *testing.T) {
	stub := initRolesStub(t)
	guest := []interface{}{constvars.KonsulinRoleGuest}
	for _, tc := range []accessTokenCase{
		{name: "role lookup fails", roles: rolesOK(), rolesErr: errors.New("roles unavailable"), wantRoles: guest},
		{name: "role response absent", wantRoles: guest},
		{name: "patient ID", roles: rolesOK(constvars.KonsulinRolePatient), lookup: &contracts.InitializeNewUserFHIRResourcesOutput{PatientID: "p1"}, wantRoles: []interface{}{constvars.KonsulinRolePatient}, wantID: "Patient/p1"},
		{name: "FHIR error does not block session", roles: rolesOK(constvars.KonsulinRolePatient), lookup: &contracts.InitializeNewUserFHIRResourcesOutput{}, lookupErr: errors.New("FHIR unavailable"), wantRoles: []interface{}{constvars.KonsulinRolePatient}},
		{name: "no FHIR resources", roles: rolesOK(constvars.KonsulinRoleSuperadmin), lookup: &contracts.InitializeNewUserFHIRResourcesOutput{}, wantRoles: []interface{}{constvars.KonsulinRoleSuperadmin}},
		{name: "empty roles still perform lookup", roles: rolesOK(), lookup: &contracts.InitializeNewUserFHIRResourcesOutput{}, wantRoles: []interface{}{}},
	} {
		t.Run(tc.name, func(t *testing.T) { runAccessTokenCase(t, stub, tc) })
	}
}

func runAccessTokenCase(t *testing.T, stub *rolesStub, tc accessTokenCase) {
	stub.response, stub.err = userrolesmodels.GetRolesForUserResponse{OK: tc.roles}, tc.rolesErr
	initializer := new(MockUserFHIRInitializer)
	if tc.lookup != nil {
		initializer.On("LookupUserFHIRResourceIDs", mock.Anything, &contracts.LookupUserFHIRResourceIDsInput{SuperTokenUserID: "user"}).Return(tc.lookup, tc.lookupErr).Once()
	}
	uc := &authUsecase{UserFHIRInitializer: initializer, Log: zap.NewNop()}
	payload := map[string]interface{}{"custom": true, supertokenAccessTokenPayloadFhirResourceId: "Patient/stale"}
	uc.buildAccessTokenPayload(context.Background(), "user", "tenant", payload)
	require.Equal(t, "user", stub.userID)
	require.Equal(t, "tenant", stub.tenant)
	require.Equal(t, true, payload["custom"])
	require.Equal(t, tc.wantID, payload[supertokenAccessTokenPayloadFhirResourceId])
	require.Equal(t, map[string]interface{}{supertokenAccessTokenPayloadRolesValueKey: tc.wantRoles}, payload[supertokenAccessTokenPayloadRolesKey])
	initializer.AssertExpectations(t)
}

func TestSessionConfigEnrichesPayloadAndForwardsArguments(t *testing.T) {
	uc := &authUsecase{Log: zap.NewNop()}
	secure, sameSite := true, "strict"
	config := uc.buildSessionConfig(&sameSite, &secure)
	require.Same(t, &sameSite, config.CookieSameSite)
	require.Same(t, &secure, config.CookieSecure)
	for _, initialPayload := range []map[string]interface{}{nil, {"custom": "value"}} {
		database := map[string]interface{}{"stored": true}
		antiCSRF := false
		userContext := &map[string]interface{}{"context": true}
		wantSession := &sessmodels.TypeSessionContainer{}
		wantErr := errors.New("session creation failed")
		called := false
		create := func(userID string, payload, data map[string]interface{}, anti *bool, tenant string, ctx supertokens.UserContext) (sessmodels.SessionContainer, error) {
			called = true
			require.Empty(t, userID)
			require.Equal(t, "tenant", tenant)
			require.Equal(t, database, data)
			require.Same(t, &antiCSRF, anti)
			require.Same(t, userContext, ctx)
			require.Equal(t, "", payload[supertokenAccessTokenPayloadFhirResourceId])
			require.Equal(t, map[string]interface{}{supertokenAccessTokenPayloadRolesValueKey: []interface{}{constvars.KonsulinRoleGuest}}, payload[supertokenAccessTokenPayloadRolesKey])
			if initialPayload != nil {
				require.Equal(t, "value", payload["custom"])
			}
			return wantSession, wantErr
		}
		refresh := func(_ string, _ *string, _ bool, _ supertokens.UserContext) (sessmodels.SessionContainer, error) {
			return nil, nil
		}
		override := config.Override.Functions(sessmodels.RecipeInterface{CreateNewSession: &create, RefreshSession: &refresh})
		got, err := (*override.CreateNewSession)("", initialPayload, database, &antiCSRF, "tenant", userContext)
		require.True(t, called)
		require.Same(t, wantSession, got)
		require.ErrorIs(t, err, wantErr)
	}
}

func TestGuestAccessTokenPayloadPreservesOtherClaims(t *testing.T) {
	payload := map[string]interface{}{"custom": "value", supertokenAccessTokenPayloadFhirResourceId: "Patient/stale"}
	uc := &authUsecase{Log: zap.NewNop()}
	uc.buildAccessTokenPayload(context.Background(), "", "public", payload)
	require.Equal(t, "value", payload["custom"])
	require.Equal(t, "", payload[supertokenAccessTokenPayloadFhirResourceId])
	require.Equal(t, map[string]interface{}{supertokenAccessTokenPayloadRolesValueKey: []interface{}{constvars.KonsulinRoleGuest}}, payload[supertokenAccessTokenPayloadRolesKey])
}

func TestSessionRefreshUpdatesAuthClaims(t *testing.T) {
	stub := initRolesStub(t)
	refreshErr := errors.New("refresh failed")
	mergeErr := errors.New("token regeneration failed")
	for _, tc := range []struct {
		name       string
		refreshErr error
		mergeErr   error
		nilSession bool
		lookupErr  error
		wantID     string
	}{
		{name: "current resource replaces stale claim", wantID: "Practitioner/new"},
		{name: "lookup failure clears stale claim", lookupErr: errors.New("FHIR unavailable")},
		{name: "refresh failure", refreshErr: refreshErr},
		{name: "nil session", nilSession: true},
		{name: "merge failure", mergeErr: mergeErr, wantID: "Practitioner/new"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stub.response, stub.err = userrolesmodels.GetRolesForUserResponse{OK: rolesOK(constvars.KonsulinRolePractitioner)}, nil
			initializer := new(MockUserFHIRInitializer)
			if tc.refreshErr == nil && !tc.nilSession {
				initializer.On("LookupUserFHIRResourceIDs", mock.Anything, &contracts.LookupUserFHIRResourceIDsInput{SuperTokenUserID: "user"}).Return(&contracts.InitializeNewUserFHIRResourcesOutput{PractitionerID: "new"}, tc.lookupErr).Once()
			}
			uc := &authUsecase{UserFHIRInitializer: initializer, Log: zap.NewNop()}
			userContext := &map[string]interface{}{"request": true}
			antiCSRF := "csrf"
			merged := false
			payload := map[string]interface{}{
				"custom":                                   "preserved",
				constvars.SupertokenPayloadActiveRoleKey:   constvars.KonsulinRolePatient,
				supertokenAccessTokenPayloadFhirResourceId: "Patient/stale",
				supertokenAccessTokenPayloadRolesKey:       map[string]interface{}{supertokenAccessTokenPayloadRolesValueKey: []interface{}{constvars.KonsulinRolePatient}},
			}
			sess := &sessmodels.TypeSessionContainer{
				GetUserIDWithContext: func(ctx supertokens.UserContext) string {
					require.Same(t, userContext, ctx)
					return "user"
				},
				GetTenantIdWithContext: func(ctx supertokens.UserContext) string {
					require.Same(t, userContext, ctx)
					return "tenant"
				},
				MergeIntoAccessTokenPayloadWithContext: func(update map[string]interface{}, ctx supertokens.UserContext) error {
					merged = true
					require.Same(t, userContext, ctx)
					require.Equal(t, tc.wantID, update[supertokenAccessTokenPayloadFhirResourceId])
					require.Equal(t, map[string]interface{}{supertokenAccessTokenPayloadRolesValueKey: []interface{}{constvars.KonsulinRolePractitioner}}, update[supertokenAccessTokenPayloadRolesKey])
					for key, value := range update {
						payload[key] = value
					}
					return tc.mergeErr
				},
			}
			refresh := func(token string, csrf *string, disable bool, ctx supertokens.UserContext) (sessmodels.SessionContainer, error) {
				require.Equal(t, "refresh-token", token)
				require.Same(t, &antiCSRF, csrf)
				require.True(t, disable)
				require.Same(t, userContext, ctx)
				if tc.nilSession || tc.refreshErr != nil {
					return nil, tc.refreshErr
				}
				return sess, nil
			}
			create := func(_ string, _, _ map[string]interface{}, _ *bool, _ string, _ supertokens.UserContext) (sessmodels.SessionContainer, error) {
				return nil, nil
			}
			override := uc.buildSessionConfig(nil, nil).Override.Functions(sessmodels.RecipeInterface{CreateNewSession: &create, RefreshSession: &refresh})
			got, err := (*override.RefreshSession)("refresh-token", &antiCSRF, true, userContext)
			switch {
			case tc.refreshErr != nil:
				require.ErrorIs(t, err, refreshErr)
				require.Nil(t, got)
			case tc.mergeErr != nil:
				require.ErrorIs(t, err, mergeErr)
				require.Nil(t, got)
			case tc.nilSession:
				require.NoError(t, err)
				require.Nil(t, got)
			default:
				require.NoError(t, err)
				require.Same(t, sess, got)
			}
			require.Equal(t, tc.refreshErr == nil && !tc.nilSession, merged)
			require.Equal(t, "preserved", payload["custom"])
			require.Equal(t, constvars.KonsulinRolePatient, payload[constvars.SupertokenPayloadActiveRoleKey])
			initializer.AssertExpectations(t)
		})
	}
}

func TestSessionLookupsInheritRequestCancellation(t *testing.T) {
	stub := initRolesStub(t)
	stub.response = userrolesmodels.GetRolesForUserResponse{OK: rolesOK(constvars.KonsulinRolePatient)}
	for _, mode := range []string{"create", "refresh"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			deadline, _ := ctx.Deadline()
			request := httptest.NewRequest(http.MethodPost, "/auth", http.NoBody).WithContext(ctx)
			userContext := supertokens.SetRequestInUserContextIfNotDefined(nil, request)
			cancel()
			initializer := new(MockUserFHIRInitializer)
			initializer.On("LookupUserFHIRResourceIDs", mock.MatchedBy(func(got context.Context) bool {
				gotDeadline, ok := got.Deadline()
				return ok && gotDeadline.Equal(deadline) && got.Err() == context.Canceled
			}), &contracts.LookupUserFHIRResourceIDsInput{SuperTokenUserID: "user"}).Return(nil, context.Canceled).Once()
			uc := &authUsecase{UserFHIRInitializer: initializer, Log: zap.NewNop()}
			var claims map[string]interface{}
			sess := &sessmodels.TypeSessionContainer{
				GetUserIDWithContext:   func(_ supertokens.UserContext) string { return "user" },
				GetTenantIdWithContext: func(_ supertokens.UserContext) string { return "tenant" },
				MergeIntoAccessTokenPayloadWithContext: func(update map[string]interface{}, _ supertokens.UserContext) error {
					claims = update
					return nil
				},
			}
			create := func(_ string, payload, _ map[string]interface{}, _ *bool, _ string, _ supertokens.UserContext) (sessmodels.SessionContainer, error) {
				claims = payload
				return sess, nil
			}
			refresh := func(_ string, _ *string, _ bool, _ supertokens.UserContext) (sessmodels.SessionContainer, error) {
				return sess, nil
			}
			override := uc.buildSessionConfig(nil, nil).Override.Functions(sessmodels.RecipeInterface{CreateNewSession: &create, RefreshSession: &refresh})
			if mode == "create" {
				_, err := (*override.CreateNewSession)("user", nil, nil, nil, "tenant", userContext)
				require.NoError(t, err)
			} else {
				_, err := (*override.RefreshSession)("refresh-token", nil, true, userContext)
				require.NoError(t, err)
			}
			require.Equal(t, "", claims[supertokenAccessTokenPayloadFhirResourceId])
			initializer.AssertExpectations(t)
		})
	}
}
