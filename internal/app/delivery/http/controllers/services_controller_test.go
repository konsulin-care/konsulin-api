package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/app/delivery/http/middlewares"
	"konsulin-service/internal/app/services/core/services"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/fhir_dto"

	"github.com/go-chi/chi/v5"
	"github.com/konsulin-care/gateway-sdk-go/wire"
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
// 404 path never calls the gateway. The sync members back the T36B inline
// route: syncReply is the opened reply, syncErr the gateway failure (e.g.
// a 504 wire error) the handler must pass through.
type servicesFakeHooks struct {
	enqueued    []string
	syncCalls   int
	syncTargets []string
	syncReply   []byte
	syncErr     error
}

// Enqueue records the recipient and service of one async invocation.
func (f *servicesFakeHooks) Enqueue(_ context.Context, to, service, _ string, _ []byte) error {
	f.enqueued = append(f.enqueued, to+"/"+service)
	return nil
}

// Sync counts the inline invocation, records the recipient and service so
// a test can prove which listing ran, and answers the configured reply or
// error so a test can prove the opened reply is returned and a gateway 504
// keeps its status.
func (f *servicesFakeHooks) Sync(_ context.Context, to, service, _ string, _ []byte) ([]byte, error) {
	f.syncCalls++
	f.syncTargets = append(f.syncTargets, to+"/"+service)
	return f.syncReply, f.syncErr
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
		services.NewResultUsecase(&servicesFakeRequests{}, nil, zap.NewNop()),
		gatewayEnv, &config.InternalConfig{})
	ctrl.Enforcer = controllerTestEnforcer(t)
	return ctrl, dir, hooks
}

// newServicesTestMux mirrors the attachServicesRouter wiring: every
// services route behind RequireGateway on a bare chi mux, with an enabled
// gateway status so requests reach the handlers.
func newServicesTestMux(ctrl *ServicesController) *chi.Mux {
	return newServicesTestMuxWithStatus(ctrl, servicesFakeGatewayStatus{})
}

// newServicesTestMuxWithStatus mounts the same routes behind RequireGateway
// over an arbitrary gateway status (disabled = standalone mode).
func newServicesTestMuxWithStatus(ctrl *ServicesController, status contracts.GatewayStatus) *chi.Mux {
	mux := chi.NewRouter()
	r := mux.With(middlewares.RequireGateway(status))
	r.Get("/services", ctrl.ListServices)
	r.Post("/services/{provider}/{name}", ctrl.InvokeService)
	r.Post("/services/{provider}/{name}/sync", ctrl.SyncService)
	r.Get("/services/requests/{id}", ctrl.GetServiceRequest)
	return mux
}

// servicesRequest builds a request carrying the caller roles and uid the
// handlers read from context.
func servicesRequest(method, target, body string, roles ...string) *http.Request {
	return servicesRequestAs("uid-1", method, target, body, roles...)
}

// servicesRequestAs is servicesRequest with an explicit caller uid, so a
// test can act as a caller other than the request owner.
func servicesRequestAs(uid, method, target, body string, roles ...string) *http.Request {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	ctx := context.WithValue(req.Context(), constvars.CONTEXT_FHIR_ROLE, roles)
	ctx = context.WithValue(ctx, constvars.CONTEXT_UID, uid)
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

// servicesFakeRequests is a contracts.ServiceRequestFhirClient double
// serving seeded ServiceRequests from Search, so the T36B status handler
// runs the real ResultUsecase owner check over it.
type servicesFakeRequests struct {
	stored []fhir_dto.GetServiceRequestOutput
}

var _ contracts.ServiceRequestFhirClient = (*servicesFakeRequests)(nil)

// Search serves the seeded resources filtered by input id.
func (f *servicesFakeRequests) Search(_ context.Context, input *fhir_dto.SearchServiceRequestInput) ([]fhir_dto.GetServiceRequestOutput, error) {
	out := make([]fhir_dto.GetServiceRequestOutput, 0, len(f.stored))
	for _, sr := range f.stored {
		if input.ID == "" || sr.ID == input.ID {
			out = append(out, sr)
		}
	}
	return out, nil
}

// CreateServiceRequest fails: these tests never create receipts.
func (*servicesFakeRequests) CreateServiceRequest(context.Context, *fhir_dto.CreateServiceRequestInput) (*fhir_dto.CreateServiceRequestOutput, error) {
	return nil, errors.New("services fake: create not expected")
}

// GetServiceRequestByIDAndVersion is never exercised by these tests.
func (*servicesFakeRequests) GetServiceRequestByIDAndVersion(context.Context, string, string) (*fhir_dto.GetServiceRequestOutput, error) {
	return nil, errors.New("services fake: version read not expected")
}

// Update is never exercised by these tests.
func (*servicesFakeRequests) Update(context.Context, string, *fhir_dto.UpdateServiceRequestInput) (*fhir_dto.GetServiceRequestOutput, error) {
	return nil, errors.New("services fake: update not expected")
}

// EnsureAllNecessaryGroupsExists is never exercised by these tests.
func (*servicesFakeRequests) EnsureAllNecessaryGroupsExists(context.Context) error { return nil }

// servicesDisabledGatewayStatus reports a disabled gateway (standalone
// mode) so RequireGateway answers 503 before any handler runs.
type servicesDisabledGatewayStatus struct{}

// Enabled reports the gateway as not configured.
func (servicesDisabledGatewayStatus) Enabled() bool { return false }

// Enrolled reports the party as not enrolled.
func (servicesDisabledGatewayStatus) Enrolled() bool { return false }

// TestSyncServiceReturnsOpenedReplyAndPassesGateway504 covers the first
// T36B assertion: an authorized sync POST answers 200 with the opened
// reply body, a denied one fails closed 404 without a gateway call, and a
// gateway 504 keeps its status.
func TestSyncServiceReturnsOpenedReplyAndPassesGateway504(t *testing.T) {
	// BuildErrorResponse only exposes dev_message outside production.
	t.Setenv("APP_ENV", "development")
	ctrl, _, hooks := newServicesTestFixture(t)
	mux := newServicesTestMux(ctrl)

	hooks.syncReply = []byte(`{"answer":42}`)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, servicesRequest(http.MethodPost,
		"/services/automation/interpret/sync", `{"prompt":"x"}`, "Guest"))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var payload struct {
		Success bool            `json:"success"`
		Data    json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	require.True(t, payload.Success)
	assert.JSONEq(t, `{"answer":42}`, string(payload.Data),
		"sync must return the opened reply body")
	require.Equal(t, 1, hooks.syncCalls, "exactly one inline invocation")

	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, servicesRequest(http.MethodPost,
		"/services/automation/send-wa-link/sync", `{}`, "Guest"))
	require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	assert.Equal(t, 1, hooks.syncCalls, "a denied sync must not reach the gateway")

	hooks.syncReply = nil
	hooks.syncErr = &wire.Error{
		Code: "GW-SRV-005", Message: "upstream timeout",
		RequestID: "gw-req-9", Status: http.StatusGatewayTimeout,
	}
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, servicesRequest(http.MethodPost,
		"/services/automation/interpret/sync", `{"prompt":"x"}`, "Guest"))
	require.Equal(t, http.StatusGatewayTimeout, rec.Code, rec.Body.String())
	assert.Equal(t, 2, hooks.syncCalls, "the failing sync must reach the gateway once")
}

// TestGetServiceRequestReturnsOwnerViewAnd404ToOthers covers the second
// T36B assertion: the request owner reads {status, result}, any other
// caller gets 404.
func TestGetServiceRequestReturnsOwnerViewAnd404ToOthers(t *testing.T) {
	ctrl, _, _ := newServicesTestFixture(t)
	ctrl.Results = services.NewResultUsecase(&servicesFakeRequests{stored: []fhir_dto.GetServiceRequestOutput{{
		ResourceType: constvars.ResourceServiceRequest,
		ID:           "sr-1",
		Status:       "completed",
		Requester:    fhir_dto.Reference{Reference: "uid-1"},
		Note:         []fhir_dto.Annotation{{Text: `{"answer":42}`}},
	}}}, nil, zap.NewNop())
	mux := newServicesTestMux(ctrl)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, servicesRequest(http.MethodGet, "/services/requests/sr-1", "", "Guest"))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var payload struct {
		Success bool `json:"success"`
		Data    struct {
			Status string `json:"status"`
			Result string `json:"result"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	require.True(t, payload.Success)
	assert.Equal(t, "completed", payload.Data.Status, "the owner must get the request status")
	assert.Equal(t, `{"answer":42}`, payload.Data.Result, "the owner must get the stored result")

	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, servicesRequestAs("uid-2", http.MethodGet,
		"/services/requests/sr-1", "", "Guest"))
	require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
}

// TestServicesRoutesStandaloneAnswer503NotConfigured covers the third T36B
// assertion: with the gateway disabled every /api/v1/services route answers
// 503 with dev_message GATEWAY_NOT_CONFIGURED because RequireGateway runs
// on the mount before any handler.
func TestServicesRoutesStandaloneAnswer503NotConfigured(t *testing.T) {
	// BuildErrorResponse only exposes dev_message outside production.
	t.Setenv("APP_ENV", "development")
	ctrl, _, _ := newServicesTestFixture(t)
	mux := newServicesTestMuxWithStatus(ctrl, servicesDisabledGatewayStatus{})

	for _, tc := range []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/services"},
		{http.MethodPost, "/services/automation/interpret"},
		{http.MethodPost, "/services/automation/interpret/sync"},
		{http.MethodGet, "/services/requests/sr-1"},
	} {
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(""))
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		require.Equal(t, http.StatusServiceUnavailable, w.Code,
			tc.path+": standalone mode must answer 503")
		var envelope exceptions.CustomError
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &envelope),
			tc.path+": the error response must be a CustomError JSON envelope")
		assert.Equal(t, "GATEWAY_NOT_CONFIGURED", envelope.DevMessage,
			tc.path+": dev_message must be the GATEWAY_NOT_CONFIGURED code")
	}
}

// newServicesTestHookMux mirrors routers.attachWebhookRouter's temporary
// T38 alias wiring: the ServicesController serves POST /hook/interpret
// (async), POST /hook/synchronous/{service} (allowlisted sync), the
// fail-closed POST /hook/{service} wildcard and the owner-only result view
// at GET /service-request/{id}/result. The legacy WebhookController
// bindings and POST /callback/service-request are gone.
func newServicesTestHookMux(ctrl *ServicesController) *chi.Mux {
	mux := chi.NewRouter()
	mux.Post("/hook/interpret", ctrl.HookInterpret)
	mux.Post("/hook/synchronous/{service}", ctrl.HookSynchronous)
	mux.Post("/hook/{service}", ctrl.HookUnknown)
	mux.Get("/service-request/{id}/result", ctrl.GetServiceRequest)
	return mux
}

// TestHookInterpretAliasAnswers202WithRequestID covers the first T38
// assertion: the temporary POST /hook/interpret alias runs the async
// automation/interpret invocation and answers 202 with the receipt as
// requestId.
func TestHookInterpretAliasAnswers202WithRequestID(t *testing.T) {
	ctrl, dir, hooks := newServicesTestFixture(t)
	mux := newServicesTestHookMux(ctrl)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, servicesRequest(http.MethodPost, "/hook/interpret",
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
		"the alias must invoke automation/interpret")
	assert.Equal(t, 1, dir.findCalls, "the listing must be resolved exactly once")
}

// TestHookSynchronousModifyProfileRunsSyncInvocation covers the second T38
// assertion: POST /hook/synchronous/modify-profile performs the inline
// sync automation/modify-profile invocation and returns its opened reply.
func TestHookSynchronousModifyProfileRunsSyncInvocation(t *testing.T) {
	ctrl, _, hooks := newServicesTestFixture(t)
	hooks.syncReply = []byte(`{"saved":true}`)
	mux := newServicesTestHookMux(ctrl)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, servicesRequest(http.MethodPost,
		"/hook/synchronous/modify-profile", `{"name":"x"}`, "Guest"))

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var payload struct {
		Success bool            `json:"success"`
		Data    json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	require.True(t, payload.Success)
	assert.JSONEq(t, `{"saved":true}`, string(payload.Data),
		"the alias must return the opened reply body")
	require.Equal(t, 1, hooks.syncCalls, "exactly one sync invocation")
	require.Len(t, hooks.syncTargets, 1)
	assert.Equal(t, servicesAutomationParty+"/modify-profile", hooks.syncTargets[0],
		"the alias must sync the automation modify-profile listing")
	assert.Empty(t, hooks.enqueued, "the alias must not enqueue an async invocation")
}

// TestUnmappedHookServiceNameAnswers404 covers the third T38 assertion: a
// /hook name outside the alias map fails closed 404 — both the wildcard
// route and a synchronous service outside the allowlist — without any
// gateway invocation.
func TestUnmappedHookServiceNameAnswers404(t *testing.T) {
	ctrl, _, hooks := newServicesTestFixture(t)
	mux := newServicesTestHookMux(ctrl)

	for _, target := range []string{
		"/hook/send-magiclink",
		"/hook/notify-provider",
		"/hook/synchronous/send-magiclink",
	} {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, servicesRequest(http.MethodPost, target, `{}`, "Guest"))
		require.Equal(t, http.StatusNotFound, rec.Code, target)
	}
	assert.Empty(t, hooks.enqueued, "no async invocation may happen on a 404")
	assert.Zero(t, hooks.syncCalls, "no sync invocation may happen on a 404")
}

// TestServiceRequestResultAliasServesOwnerView pins the fourth T38 alias:
// GET /service-request/{id}/result is the owner-only result view served by
// the ServicesController's GetServiceRequest.
func TestServiceRequestResultAliasServesOwnerView(t *testing.T) {
	ctrl, _, _ := newServicesTestFixture(t)
	ctrl.Results = services.NewResultUsecase(&servicesFakeRequests{stored: []fhir_dto.GetServiceRequestOutput{{
		ResourceType: constvars.ResourceServiceRequest,
		ID:           "sr-9",
		Status:       "completed",
		Requester:    fhir_dto.Reference{Reference: "uid-1"},
		Note:         []fhir_dto.Annotation{{Text: `{"answer":7}`}},
	}}}, nil, zap.NewNop())
	mux := newServicesTestHookMux(ctrl)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, servicesRequest(http.MethodGet,
		"/service-request/sr-9/result", "", "Guest"))

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var payload struct {
		Success bool `json:"success"`
		Data    struct {
			Status string `json:"status"`
			Result string `json:"result"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	require.True(t, payload.Success)
	assert.Equal(t, "completed", payload.Data.Status, "the owner must get the request status")
	assert.Equal(t, `{"answer":7}`, payload.Data.Result, "the owner must get the stored result")
}
