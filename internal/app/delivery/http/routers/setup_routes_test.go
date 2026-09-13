package routers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/delivery/http/controllers"
	"konsulin-service/internal/app/delivery/http/middlewares"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/supertokens/supertokens-golang/recipe/passwordless"
	"github.com/supertokens/supertokens-golang/recipe/passwordless/plessmodels"
	"github.com/supertokens/supertokens-golang/recipe/session"
	"github.com/supertokens/supertokens-golang/supertokens"
	"go.uber.org/zap"
)

// setupRoutesTestConfig returns a minimal InternalConfig sufficient for
// SetupRoutes: it only needs the values read at registration time.
func setupRoutesTestConfig() *config.InternalConfig {
	return &config.InternalConfig{
		App: config.App{
			EndpointPrefix:            "api",
			Version:                   "v1",
			FrontendDomain:            "http://localhost:3000",
			SuperadminAPIKey:          "test-superadmin-api-key",
			MaxRequests:               100,
			SuperadminAPIKeyRateLimit: 100,
		},
		FHIR: config.AppFHIR{
			BaseUrl:                  "http://localhost:8080/fhir",
			TerminologyServerBaseUrl: "http://localhost:8081",
		},
	}
}

// TestSetupRoutesRegistersHealthAfterMiddlewares guards the chi ordering
// invariant: every Use() on a mux must precede any route registration on that
// same mux. Registering /health before the middleware chain panics with
// "chi: all middlewares must be defined before routes on a mux", which broke
// the server at startup after the /health endpoint was introduced.
func TestSetupRoutesRegistersHealthAfterMiddlewares(t *testing.T) {
	// supertokens.Init is required because SetupRoutes calls
	// supertokens.GetAllCORSHeaders() while building the CORS options.
	_ = supertokens.Init(supertokens.TypeInput{
		AppInfo: supertokens.AppInfo{
			AppName:       "konsulin-test",
			APIDomain:     "http://localhost:3001",
			WebsiteDomain: "http://localhost:3000",
		},
		Supertokens: &supertokens.ConnectionInfo{ConnectionURI: "http://localhost:3567"},
		RecipeList: []supertokens.Recipe{
			passwordless.Init(plessmodels.TypeInput{FlowType: "MAGIC_LINK", ContactMethodEmail: plessmodels.ContactMethodEmailConfig{Enabled: true}}),
			session.Init(nil),
		},
	})

	logger := zap.NewNop()
	cfg := setupRoutesTestConfig()
	mw := &middlewares.Middlewares{Log: logger, InternalConfig: cfg}

	router := chi.NewRouter()
	SetupRoutes(
		router,
		cfg,
		logger,
		mw,
		&controllers.AuthController{},
		&controllers.PaymentController{},
		&controllers.WebhookController{},
		&controllers.ScheduleController{},
		&controllers.OrganizationController{},
		&controllers.PurgeController{},
	)

	// The liveness probe must survive the full middleware chain: it is
	// registered after the chain, so no route-before-middleware panic.
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
	assert.JSONEq(t,
		`{"status":"OK","version":"develop","tag":"0.0.1-rc","hash":"unknown"}`,
		w.Body.String())
}
