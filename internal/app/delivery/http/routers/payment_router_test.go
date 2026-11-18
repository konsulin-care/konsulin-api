package routers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/konsulin-care/konsulin-api/internal/app/delivery/http/middlewares"
	"github.com/konsulin-care/konsulin-api/internal/pkg/exceptions"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/supertokens/supertokens-golang/recipe/passwordless"
	"github.com/supertokens/supertokens-golang/recipe/passwordless/plessmodels"
	"github.com/supertokens/supertokens-golang/recipe/session"
	"github.com/supertokens/supertokens-golang/supertokens"
	"go.uber.org/zap"
)

// initSuperTokensOnce initializes the SuperTokens SDK for the route tests if
// no instance exists yet. SetupRoutes calls supertokens.GetAllCORSHeaders,
// which panics without an instance; the recipe list mirrors
// setupRoutesTestConfig's usage in setup_routes_test.go.
// Parameters: none.
// Returns: nothing; ignores a re-init error because an existing instance is
// already sufficient.
func initSuperTokensOnce() {
	if _, err := supertokens.GetInstanceOrThrowError(); err == nil {
		return
	}
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
}

// TestPaymentRoutesStandaloneAnswer503 covers the T26B contract: in standalone
// mode (zero-value Controllers, so no gateway status) POST
// /api/v1/pay/appointment and POST /api/v1/pay/service must answer 503 with
// dev_message GATEWAY_NOT_CONFIGURED because RequireGateway rejects the
// request before the payment handler runs.
// Parameters: t is the test harness.
// Returns: nothing; fails on any deviation.
func TestPaymentRoutesStandaloneAnswer503(t *testing.T) {
	// BuildErrorResponse only exposes dev_message outside production.
	t.Setenv("APP_ENV", "development")

	initSuperTokensOnce()

	logger := zap.NewNop()
	cfg := setupRoutesTestConfig()
	mw := &middlewares.Middlewares{Log: logger, InternalConfig: cfg}

	router := chi.NewRouter()
	// Zero-value Controllers: nil Gateway status counts as disabled, which is
	// exactly the standalone deployment this assertion describes.
	SetupRoutes(router, cfg, logger, mw, Controllers{})

	for _, path := range []string{"/api/v1/pay/appointment", "/api/v1/pay/service"} {
		req := httptest.NewRequest(http.MethodPost, path, nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusServiceUnavailable, w.Code,
			path+": standalone mode must answer 503")

		var envelope exceptions.CustomError
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &envelope),
			path+": the error response must be a CustomError JSON envelope")
		assert.Equal(t, "GATEWAY_NOT_CONFIGURED", envelope.DevMessage,
			path+": dev_message must be the GATEWAY_NOT_CONFIGURED code")
	}
}
