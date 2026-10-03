package auth

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"konsulin-service/internal/app/config"
	"konsulin-service/internal/pkg/constvars"

	"github.com/stretchr/testify/require"
	"github.com/supertokens/supertokens-golang/recipe/passwordless"
	"github.com/supertokens/supertokens-golang/recipe/passwordless/plessmodels"
	"github.com/supertokens/supertokens-golang/recipe/session"
	"github.com/supertokens/supertokens-golang/recipe/userroles"
	"github.com/supertokens/supertokens-golang/recipe/userroles/userrolesmodels"
	"github.com/supertokens/supertokens-golang/supertokens"
	"go.uber.org/zap"
)

// fakeCore stands in for the SuperTokens core endpoints used while initializing roles.
type fakeCore struct {
	mu             sync.Mutex
	createdNewRole bool
	failRoles      bool
	roles          []string
}

func (c *fakeCore) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c.mu.Lock()
	defer c.mu.Unlock()
	w.Header().Set(constvars.HeaderContentType, constvars.MIMEApplicationJSON)
	switch {
	case r.URL.Path == "/apiversion":
		_ = json.NewEncoder(w).Encode(map[string][]string{"versions": {"3.1"}})
	case r.Method == http.MethodPut && r.URL.Path == "/recipe/role" && !c.failRoles:
		var body struct {
			Role string `json:"role"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		c.roles = append(c.roles, body.Role)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "OK", "createdNewRole": c.createdNewRole})
	default:
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func (c *fakeCore) requestedRoles() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.roles
}

var allKonsulinRoles = []string{
	constvars.KonsulinRolePatient,
	constvars.KonsulinRoleGuest,
	constvars.KonsulinRoleClinicAdmin,
	constvars.KonsulinRolePractitioner,
	constvars.KonsulinRoleResearcher,
	constvars.KonsulinRoleSuperadmin,
}

type initializeCase struct {
	name      string
	env       string
	apiDomain string
	core      *fakeCore
	wantRoles []string
	wantErr   string
}

func TestInitializeSupertoken(t *testing.T) {
	for _, tc := range []initializeCase{
		{name: "creates every role", env: "local", apiDomain: "http://localhost:3200", core: &fakeCore{createdNewRole: true}, wantRoles: allKonsulinRoles},
		{name: "existing roles are accepted", env: "production", apiDomain: "https://api.example.test", core: &fakeCore{}, wantRoles: allKonsulinRoles},
		{name: "role creation failure aborts startup", env: "development", apiDomain: "http://localhost:3200", core: &fakeCore{failRoles: true}, wantErr: `ensure supertokens role "Patient" exists`},
		{name: "invalid app info", env: "local", core: &fakeCore{}, wantErr: "apiDomain"},
	} {
		t.Run(tc.name, func(t *testing.T) { runInitializeCase(t, tc) })
	}
}

func runInitializeCase(t *testing.T, tc initializeCase) {
	supertokens.ResetForTest()
	t.Cleanup(supertokens.ResetForTest)
	core := httptest.NewServer(tc.core)
	t.Cleanup(core.Close)
	uc := &authUsecase{
		Log: zap.NewNop(),
		InternalConfig: &config.InternalConfig{
			App:        config.App{Env: tc.env, EndpointPrefix: "/api", Version: "v1"},
			Supertoken: config.AppSupertoken{KonsulinDasboardAdminEmail: "admin@example.test"},
		},
		DriverConfig: &config.DriverConfig{Supertoken: config.Supertoken{
			ApiBasePath: "/auth", WebsiteBasePath: "/auth", ConnectionURI: core.URL,
			AppName: "konsulin-test", ApiDomain: tc.apiDomain, WebsiteDomain: "http://localhost:3000",
		}},
	}
	requireErrorContains(t, tc.wantErr, uc.InitializeSupertoken())
	require.Equal(t, tc.wantRoles, tc.core.requestedRoles())
}

func requireErrorContains(t *testing.T, want string, err error) {
	t.Helper()
	if want == "" {
		require.NoError(t, err)
		return
	}
	require.ErrorContains(t, err, want)
}

// createCodeStub holds what the overridden passwordless and user-role recipe functions
// return, and which user lookup ran.
type createCodeStub struct {
	user     *plessmodels.User
	userErr  error
	roles    *struct{ Roles []string }
	rolesErr error
	lookup   string
}

func initCreateCodeStub(t *testing.T) *createCodeStub {
	t.Helper()
	supertokens.ResetForTest()
	t.Cleanup(supertokens.ResetForTest)
	stub := &createCodeStub{}
	telemetry := false
	require.NoError(t, supertokens.Init(supertokens.TypeInput{
		Supertokens: &supertokens.ConnectionInfo{ConnectionURI: "http://localhost:9999"},
		AppInfo:     supertokens.AppInfo{AppName: "create-code-test", APIDomain: "http://localhost:8080", WebsiteDomain: "http://localhost:3000"},
		Telemetry:   &telemetry,
		RecipeList: []supertokens.Recipe{
			session.Init(nil),
			passwordless.Init(plessmodels.TypeInput{
				FlowType:                  "MAGIC_LINK",
				ContactMethodEmailOrPhone: plessmodels.ContactMethodEmailOrPhoneConfig{Enabled: true},
				Override:                  &plessmodels.OverrideStruct{Functions: stub.overrideUsers},
			}),
			userroles.Init(&userrolesmodels.TypeInput{Override: &userrolesmodels.OverrideStruct{Functions: stub.overrideRoles}}),
		},
	}))
	return stub
}

func (s *createCodeStub) overrideUsers(original plessmodels.RecipeInterface) plessmodels.RecipeInterface {
	*original.GetUserByEmail = func(email, tenant string, _ supertokens.UserContext) (*plessmodels.User, error) {
		s.lookup = "email:" + email + "@" + tenant
		return s.user, s.userErr
	}
	*original.GetUserByPhoneNumber = func(phone, tenant string, _ supertokens.UserContext) (*plessmodels.User, error) {
		s.lookup = "phone:" + phone + "@" + tenant
		return s.user, s.userErr
	}
	return original
}

func (s *createCodeStub) overrideRoles(original userrolesmodels.RecipeInterface) userrolesmodels.RecipeInterface {
	*original.GetRolesForUser = func(_, _ string, _ supertokens.UserContext) (userrolesmodels.GetRolesForUserResponse, error) {
		return userrolesmodels.GetRolesForUserResponse{OK: s.roles}, s.rolesErr
	}
	return original
}

type createCodeLookup struct {
	email, phone, userID string
	roles                []string
}

type createCodeCase struct {
	name         string
	email, phone *string
	stub         createCodeStub
	wantLookup   string
	want         createCodeLookup
	wantErr      string
}

func TestLookupUserForCreateCode(t *testing.T) {
	email, phone := "user@example.test", "+62 812-3456-789"
	lookupErr := errors.New("core unavailable")
	for _, tc := range []createCodeCase{
		{name: "existing user keeps fetched roles", email: &email, stub: createCodeStub{user: &plessmodels.User{ID: "u1"}, roles: rolesOK(constvars.KonsulinRolePractitioner)}, wantLookup: "email:" + email + "@tenant", want: createCodeLookup{email: email, userID: "u1", roles: []string{constvars.KonsulinRolePractitioner}}},
		{name: "user without roles defaults to Patient", phone: &phone, stub: createCodeStub{user: &plessmodels.User{ID: "u2"}, roles: rolesOK()}, wantLookup: "phone:628123456789@tenant", want: createCodeLookup{phone: "628123456789", userID: "u2", roles: []string{constvars.KonsulinRolePatient}}},
		{name: "absent roles response defaults to Patient", email: &email, stub: createCodeStub{user: &plessmodels.User{ID: "u3"}}, wantLookup: "email:" + email + "@tenant", want: createCodeLookup{email: email, userID: "u3", roles: []string{constvars.KonsulinRolePatient}}},
		{name: "first-time user defaults to Patient", email: &email, wantLookup: "email:" + email + "@tenant", want: createCodeLookup{email: email, roles: []string{constvars.KonsulinRolePatient}}},
		{name: "email lookup fails", email: &email, stub: createCodeStub{userErr: lookupErr}, wantLookup: "email:" + email + "@tenant", wantErr: lookupErr.Error()},
		{name: "phone lookup fails", phone: &phone, stub: createCodeStub{userErr: lookupErr}, wantLookup: "phone:628123456789@tenant", wantErr: lookupErr.Error()},
		{name: "role lookup fails", email: &email, stub: createCodeStub{user: &plessmodels.User{ID: "u1"}, rolesErr: lookupErr}, wantLookup: "email:" + email + "@tenant", wantErr: lookupErr.Error()},
		{name: "contact is required", wantErr: "either email or phone number is required"},
	} {
		t.Run(tc.name, func(t *testing.T) { runCreateCodeCase(t, tc) })
	}
}

func runCreateCodeCase(t *testing.T, tc createCodeCase) {
	stub := initCreateCodeStub(t)
	*stub = tc.stub
	uc := &authUsecase{Log: zap.NewNop(), InternalConfig: &config.InternalConfig{Supertoken: config.AppSupertoken{KonsulinTenantID: "tenant"}}}
	var got createCodeLookup
	var err error
	got.email, got.phone, got.userID, got.roles, err = uc.lookupUserForCreateCode(tc.email, tc.phone, "628123456789")
	require.Equal(t, tc.wantLookup, stub.lookup)
	requireErrorContains(t, tc.wantErr, err)
	if tc.wantErr == "" {
		require.Equal(t, tc.want, got)
	}
}
