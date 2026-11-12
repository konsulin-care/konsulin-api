package controllers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/app/delivery/http/middlewares"
	"konsulin-service/internal/app/services/core/services"
	"konsulin-service/internal/pkg/constvars"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// Provider and listing fixtures: the automation party is addressed by its
// "automation" alias (the RBAC policy spells the POST path that way), the
// foreign party has no policy for any of its listings.
const (
	servicesAutomationParty = "01HZYAUTOMATION00000001"
	servicesForeignParty    = "01HZYFOREIGNPARTY00001"
)

// servicesFakeDirectory is an in-memory stand-in for the T30 cached
// listing directory: it counts lookups so tests can prove which seams ran.
type servicesFakeDirectory struct {
	listings  []contracts.Listing
	findCalls int
	listCalls int
}

// FindListing returns the row matching provider+name or
// contracts.ErrListingNotFound, counting the lookup.
func (f *servicesFakeDirectory) FindListing(_ context.Context, provider, name string) (*contracts.Listing, error) {
	f.findCalls++
	for _, row := range f.listings {
		if row.ProviderPartyID == provider && row.Name == name {
			matched := row
			return &matched, nil
		}
	}
	return nil, contracts.ErrListingNotFound
}

// ListWebhookListings returns the cached webhook rows, counting the read.
func (f *servicesFakeDirectory) ListWebhookListings(_ context.Context) ([]contracts.Listing, error) {
	f.listCalls++
	out := make([]contracts.Listing, len(f.listings))
	copy(out, f.listings)
	return out, nil
}

// Enlist is never exercised by these tests.
func (f *servicesFakeDirectory) Enlist(context.Context, contracts.EnlistRequest) (string, error) {
	return "", nil
}

// servicesFakeHooks records gateway invocations so tests can assert that a
// 404 path never calls the gateway.
type servicesFakeHooks struct {
	enqueued []string
}

// Enqueue records the recipient and service of one async invocation.
func (f *servicesFakeHooks) Enqueue(_ context.Context, to, service, _ string, _ []byte) error {
	f.enqueued = append(f.enqueued, to+"/"+service)
	return nil
}

// Sync is never exercised by these tests.
func (f *servicesFakeHooks) Sync(context.Context, string, string, string, []byte) ([]byte, error) {
	return nil, nil
}

// Status is never exercised by these tests.
func (f *servicesFakeHooks) Status(context.Context, string) (string, error) {
	return "", nil
}

// servicesFakeRecorder returns a fixed async receipt id.
type servicesFakeRecorder struct {
	id string
}

// RecordInvocation returns the fixed ServiceRequest id.
func (f *servicesFakeRecorder) RecordInvocation(context.Context, *services.InvocationServiceRequest) (string, error) {
	return f.id, nil
}

// servicesFakeGatewayStatus reports an enabled and enrolled gateway so
// RequireGateway lets requests reach the handlers.
type servicesFakeGatewayStatus struct{}

// Enabled reports the gateway as configured.
func (servicesFakeGatewayStatus) Enabled() bool { return true }

// Enrolled reports the party as enrolled.
func (servicesFakeGatewayStatus) Enrolled() bool { return true }

// newServicesTestFixture builds the controller over fake seams with the
// repo RBAC policy wired in, mirroring routers.attachServicesRouter.
func newServicesTestFixture(t *testing.T) (*ServicesController, *servicesFakeDirectory, *servicesFakeHooks) {
	t.Helper()
	dir := &servicesFakeDirectory{listings: []contracts.Listing{
		{ListingID: "lst-interpret", ProviderPartyID: servicesAutomationParty, Name: "interpret", Invocation: "webhook"},
		{ListingID: "lst-modify", ProviderPartyID: servicesAutomationParty, Name: "modify-profile", Invocation: "webhook"},
		{ListingID: "lst-wa", ProviderPartyID: servicesAutomationParty, Name: "send-wa-link", Invocation: "webhook"},
		{ListingID: "lst-secret", ProviderPartyID: servicesForeignParty, Name: "secret-service", Invocation: "webhook"},
	}}
	hooks := &servicesFakeHooks{}
	gatewayEnv := config.AppGateway{AutomationPartyID: servicesAutomationParty}
	ctrl := NewServicesController(zap.NewNop(), dir,
		services.NewProviderResolver(&gatewayEnv),
		services.NewServiceLimiter(nil),
		services.NewInvoker(hooks, dir, &servicesFakeRecorder{id: "sr-fixed-1"}),
		gatewayEnv, &config.InternalConfig{})
	ctrl.Enforcer = controllerTestEnforcer(t)
	return ctrl, dir, hooks
}

// newServicesTestMux mirrors the attachServicesRouter wiring: both routes
// behind RequireGateway on a bare chi mux.
func newServicesTestMux(ctrl *ServicesController) *chi.Mux {
	mux := chi.NewRouter()
	r := mux.With(middlewares.RequireGateway(servicesFakeGatewayStatus{}))
	r.Get("/services", ctrl.ListServices)
	r.Post("/services/{provider}/{name}", ctrl.InvokeService)
	return mux
}

// servicesRequest builds a request carrying the caller roles and uid the
// handlers read from context.
func servicesRequest(method, target, body string, roles ...string) *http.Request {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	ctx := context.WithValue(req.Context(), constvars.CONTEXT_FHIR_ROLE, roles)
	ctx = context.WithValue(ctx, constvars.CONTEXT_UID, "uid-1")
	return req.WithContext(ctx)
}

// TestListServicesReturnsOnlyListingsGrantedToCallerRoles covers the first
// assertion: GET /api/v1/services keeps only the listings whose POST path
// the caller's roles are granted by Casbin Enforce.
func TestListServicesReturnsOnlyListingsGrantedToCallerRoles(t *testing.T) {
	ctrl, dir, _ := newServicesTestFixture(t)
	mux := newServicesTestMux(ctrl)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, servicesRequest(http.MethodGet, "/services", "", "Guest"))

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var payload struct {
		Success bool `json:"success"`
		Data    []struct {
			Provider string `json:"provider"`
			Name     string `json:"name"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	require.True(t, payload.Success)

	names := make(map[string]string, len(payload.Data))
	for _, row := range payload.Data {
		names[row.Name] = row.Provider
	}
	assert.Equal(t, 1, dir.listCalls, "must read the cached webhook listings once")
	assert.Contains(t, names, "interpret", "Guest holds that POST path")
	assert.Contains(t, names, "modify-profile", "Guest holds that POST path")
	assert.Equal(t, "automation", names["interpret"], "the automation party must be addressed by alias")
	assert.NotContains(t, names, "send-wa-link", "only Superadmin holds that POST path")
	assert.NotContains(t, names, "secret-service", "no role holds a POST path for the foreign provider")
}

// TestInvokeServiceAsyncAnswers202WithRequestID covers the second
// assertion: an authorized async POST resolves the provider, invokes once
// and answers 202 with the async receipt as requestId.
func TestInvokeServiceAsyncAnswers202WithRequestID(t *testing.T) {
	ctrl, dir, hooks := newServicesTestFixture(t)
	mux := newServicesTestMux(ctrl)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, servicesRequest(http.MethodPost, "/services/automation/interpret",
		`{"prompt":"interpret this"}`, "Guest"))

	require.Equal(t, http.StatusAccepted, rec.Code, rec.Body.String())
	var payload struct {
		Success bool              `json:"success"`
		Data    map[string]string `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	require.True(t, payload.Success)
	require.Equal(t, "sr-fixed-1", payload.Data["requestId"], "must carry the async receipt id")
	require.Len(t, hooks.enqueued, 1, "exactly one async invocation")
	assert.Equal(t, servicesAutomationParty+"/interpret", hooks.enqueued[0],
		"the provider alias must resolve to the party id before invoking")
	assert.Equal(t, 1, dir.findCalls, "the listing must be resolved exactly once")
}

// TestInvokeServiceUnknownOrForbiddenAnswers404WithoutGatewayCall covers
// the third assertion: unknown or forbidden provider/name answers 404
// without any gateway invocation, and without any listing lookup when the
// deny happens before the directory seam.
func TestInvokeServiceUnknownOrForbiddenAnswers404WithoutGatewayCall(t *testing.T) {
	cases := []struct {
		name     string
		target   string
		emptyDir bool
		wantFind int
	}{
		{name: "unknown provider", target: "/services/ghost/interpret", wantFind: 0},
		{name: "reserved provider", target: "/services/send-magiclink/x", wantFind: 0},
		{name: "forbidden name", target: "/services/automation/send-wa-link", wantFind: 0},
		{name: "unknown name", target: "/services/automation/interpret", emptyDir: true, wantFind: 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl, dir, hooks := newServicesTestFixture(t)
			if tc.emptyDir {
				dir.listings = nil
			}
			mux := newServicesTestMux(ctrl)

			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, servicesRequest(http.MethodPost, tc.target, `{"x":1}`, "Guest"))

			require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
			assert.Empty(t, hooks.enqueued, "no gateway invocation may happen on a 404")
			assert.Equal(t, tc.wantFind, dir.findCalls, "the lookup seam must only run when the deny is the listing miss")
		})
	}
}
