package middlewares

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"konsulin-service/internal/pkg/constvars"

	"github.com/stretchr/testify/require"
	"github.com/supertokens/supertokens-golang/recipe/session"
	"github.com/supertokens/supertokens-golang/recipe/session/claims"
	"github.com/supertokens/supertokens-golang/recipe/session/sessmodels"
	"github.com/supertokens/supertokens-golang/supertokens"
	"go.uber.org/zap"
)

func TestExtractRolesFromAccessToken(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  map[string]interface{}
		want []string
	}{
		{name: "nil payload"},
		{name: "missing roles", raw: map[string]interface{}{}},
		{name: "wrong envelope type", raw: map[string]interface{}{constvars.SupertokenPayloadRolesKey: "Patient"}},
		{name: "missing value", raw: map[string]interface{}{constvars.SupertokenPayloadRolesKey: map[string]interface{}{}}},
		{name: "wrong list type", raw: map[string]interface{}{constvars.SupertokenPayloadRolesKey: map[string]interface{}{constvars.SupertokenPayloadRolesValueKey: "Patient"}}},
		{name: "empty list", raw: map[string]interface{}{constvars.SupertokenPayloadRolesKey: map[string]interface{}{constvars.SupertokenPayloadRolesValueKey: []interface{}{}}}, want: []string{}},
		{name: "only strings accepted", raw: map[string]interface{}{constvars.SupertokenPayloadRolesKey: map[string]interface{}{constvars.SupertokenPayloadRolesValueKey: []interface{}{constvars.KonsulinRolePatient, nil, 1, constvars.KonsulinRolePractitioner}}}, want: []string{constvars.KonsulinRolePatient, constvars.KonsulinRolePractitioner}},
	} {
		t.Run(tc.name, func(t *testing.T) { require.Equal(t, tc.want, extractRolesFromAccessToken(tc.raw)) })
	}
}

// authContext is the auth state a session middleware publishes on the request context.
type authContext struct {
	uid, resource string
	roles         []string
	activeRole    interface{}
}

var guestAuth = authContext{uid: "anonymous", roles: []string{constvars.KonsulinRoleGuest}}

// sessionStub holds what the overridden SuperTokens GetSession returns, and the
// SessionRequired option it was last called with.
type sessionStub struct {
	session  sessmodels.SessionContainer
	err      error
	required *bool
}

func initSessionStub(t *testing.T) *sessionStub {
	t.Helper()
	supertokens.ResetForTest()
	t.Cleanup(supertokens.ResetForTest)
	stub := &sessionStub{}
	telemetry := false
	require.NoError(t, supertokens.Init(supertokens.TypeInput{
		Supertokens: &supertokens.ConnectionInfo{ConnectionURI: "http://localhost:9999"},
		AppInfo:     supertokens.AppInfo{AppName: "session-test", APIDomain: "http://localhost:8080", WebsiteDomain: "http://localhost:3000"},
		Telemetry:   &telemetry,
		RecipeList: []supertokens.Recipe{session.Init(&sessmodels.TypeInput{Override: &sessmodels.OverrideStruct{Functions: func(original sessmodels.RecipeInterface) sessmodels.RecipeInterface {
			*original.GetSession = func(_, _ *string, options *sessmodels.VerifySessionOptions, _ supertokens.UserContext) (sessmodels.SessionContainer, error) {
				stub.required = options.SessionRequired
				return stub.session, stub.err
			}
			return original
		}}})},
	}))
	return stub
}

func stubSession(payload map[string]interface{}) sessmodels.SessionContainer {
	return &sessmodels.TypeSessionContainer{
		GetUserID: func() string { return "user" }, GetTenantId: func() string { return "public" },
		GetUserIDWithContext:               func(_ supertokens.UserContext) string { return "user" },
		GetTenantIdWithContext:             func(_ supertokens.UserContext) string { return "public" },
		GetAccessTokenPayload:              func() map[string]interface{} { return payload },
		AssertClaimsWithContext:            func(_ []claims.SessionClaimValidator, _ supertokens.UserContext) error { return nil },
		AttachToRequestResponseWithContext: func(_ sessmodels.RequestResponseInfo, _ supertokens.UserContext) error { return nil },
	}
}

func patientPayload(activeRole bool) map[string]interface{} {
	payload := map[string]interface{}{
		constvars.SupertokenPayloadRolesKey:          map[string]interface{}{constvars.SupertokenPayloadRolesValueKey: []interface{}{constvars.KonsulinRolePatient}},
		constvars.SupertokenPayloadFhirResourceIDKey: "Patient/p1",
	}
	if activeRole {
		payload[constvars.SupertokenPayloadActiveRoleKey] = constvars.KonsulinRolePatient
	}
	return payload
}

type sessionCase struct {
	name    string
	session sessmodels.SessionContainer
	err     error
	want    authContext
}

func TestSessionMiddlewareContext(t *testing.T) {
	stub := initSessionStub(t)
	m := &Middlewares{Log: zap.NewNop()}
	patient := authContext{uid: "user", resource: "Patient/p1", roles: []string{constvars.KonsulinRolePatient}}
	activePatient := patient
	activePatient.activeRole = constvars.KonsulinRolePatient
	for _, tc := range []sessionCase{
		{name: "anonymous", want: guestAuth},
		{name: "authenticated", session: stubSession(patientPayload(false)), want: patient},
		{name: "authenticated active role", session: stubSession(patientPayload(true)), want: activePatient},
		{name: "session error falls back to guest", err: errors.New("session unavailable"), want: guestAuth},
	} {
		t.Run(tc.name, func(t *testing.T) { runSessionCase(t, m, stub, tc) })
	}
}

// runSessionCase serves tc through each session middleware. CreateAnonymousSessionIfNeeded
// never rewrites the context, and EnsureAnonymousSession leaves authenticated requests alone.
func runSessionCase(t *testing.T, m *Middlewares, stub *sessionStub, tc sessionCase) {
	stub.session, stub.err = tc.session, tc.err
	for _, mw := range []struct {
		name        string
		middleware  func(http.Handler) http.Handler
		passThrough bool
	}{
		{"optional", m.SessionOptional, false},
		{"create", m.CreateAnonymousSessionIfNeeded, true},
		{"ensure", m.EnsureAnonymousSession, tc.session != nil},
	} {
		t.Run(mw.name, func(t *testing.T) {
			stub.required = nil
			ctx := context.Background()
			got := serveContext(t, ctx, mw.middleware)
			require.NotNil(t, stub.required)
			require.False(t, *stub.required)
			assertAuthContext(t, ctx, got, mw.passThrough, tc.want)
		})
	}
}

// serveContext runs middleware on a request carrying ctx and returns the context the
// next handler received.
func serveContext(t *testing.T, ctx context.Context, middleware func(http.Handler) http.Handler) context.Context {
	t.Helper()
	var got context.Context
	next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { got = r.Context() })
	middleware(next).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/fhir/Patient", http.NoBody).WithContext(ctx))
	require.NotNil(t, got, "next handler was not called")
	return got
}

func assertAuthContext(t *testing.T, base, got context.Context, passThrough bool, want authContext) {
	t.Helper()
	if passThrough {
		require.Equal(t, base, got)
		return
	}
	for _, key := range []interface{}{keyUID, constvars.CONTEXT_UID} {
		require.Equal(t, want.uid, got.Value(key))
	}
	for _, key := range []interface{}{keyRoles, constvars.CONTEXT_FHIR_ROLE} {
		require.Equal(t, want.roles, got.Value(key))
	}
	for _, key := range []interface{}{keyFHIRResourceID, constvars.CONTEXT_FHIR_RESOURCE_ID} {
		require.Equal(t, want.resource, got.Value(key))
	}
	require.Equal(t, want.activeRole, got.Value(keyActiveRole))
}

func TestBuildSessionAuth(t *testing.T) {
	for _, tc := range []struct {
		name                 string
		payload              map[string]interface{}
		roles                []string
		activeRole, resource string
	}{
		{name: "nil payload"},
		{name: "malformed claims", payload: map[string]interface{}{constvars.SupertokenPayloadActiveRoleKey: 123, constvars.SupertokenPayloadFhirResourceIDKey: false}},
		{name: "empty active role", payload: map[string]interface{}{constvars.SupertokenPayloadActiveRoleKey: ""}},
		{name: "complete claims", payload: map[string]interface{}{constvars.SupertokenPayloadRolesKey: map[string]interface{}{constvars.SupertokenPayloadRolesValueKey: []interface{}{constvars.KonsulinRolePatient}}, constvars.SupertokenPayloadActiveRoleKey: constvars.KonsulinRolePatient, constvars.SupertokenPayloadFhirResourceIDKey: "Patient/p1"}, roles: []string{constvars.KonsulinRolePatient}, activeRole: constvars.KonsulinRolePatient, resource: "Patient/p1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sess := &sessmodels.TypeSessionContainer{GetUserID: func() string { return "user" }, GetAccessTokenPayload: func() map[string]interface{} { return tc.payload }}
			uid, roles, activeRole, resource := buildSessionAuth(sess)
			require.Equal(t, "user", uid)
			require.Equal(t, tc.roles, roles)
			require.Equal(t, tc.activeRole, activeRole)
			require.Equal(t, tc.resource, resource)
		})
	}
}

func TestSessionMiddlewaresPreserveAPIKeyContext(t *testing.T) {
	m := &Middlewares{Log: zap.NewNop()}
	for name, middleware := range map[string]func(http.Handler) http.Handler{"optional": m.SessionOptional, "create anonymous": m.CreateAnonymousSessionIfNeeded, "ensure anonymous": m.EnsureAnonymousSession} {
		t.Run(name, func(t *testing.T) {
			ctx := context.WithValue(context.Background(), ContextAPIKeyAuth, true)
			ctx = context.WithValue(ctx, constvars.CONTEXT_UID, "api-key-user")
			ctx = context.WithValue(ctx, constvars.CONTEXT_FHIR_ROLE, []string{constvars.KonsulinRoleSuperadmin})
			called := false
			handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				require.Equal(t, ctx, r.Context())
				w.WriteHeader(http.StatusNoContent)
			}))
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/fhir/Patient", http.NoBody).WithContext(ctx))
			require.True(t, called)
			require.Equal(t, http.StatusNoContent, w.Code)
		})
	}
}
