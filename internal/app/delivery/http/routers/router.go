package routers

import (
	"encoding/json"
	"fmt"
	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/delivery/http/controllers"
	"konsulin-service/internal/app/delivery/http/middlewares"
	"konsulin-service/internal/pkg/buildinfo"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/supertokens/supertokens-golang/supertokens"
	"go.uber.org/zap"
)

func SetupRoutes(
	router *chi.Mux,
	internalConfig *config.InternalConfig,
	logger *zap.Logger,
	middlewares *middlewares.Middlewares,
	authController *controllers.AuthController,
	paymentController *controllers.PaymentController,
	webhookController *controllers.WebhookController,
	scheduleController *controllers.ScheduleController,
	organizationController *controllers.OrganizationController,
	purgeController *controllers.PurgeController,
) {
	// Liveness endpoint. Registered AFTER the middleware chain: chi requires
	// all Use() calls to precede any route registration on the same mux, so a
	// route cannot bypass the mux's own middleware stack. The chain is benign
	// for a keyless GET /health (API key and session middlewares pass through).
	corsOptions := cors.Options{
		AllowOriginFunc: func(_ *http.Request, origin string) bool {
			if strings.HasPrefix(origin, "http://localhost:") || strings.HasPrefix(origin, "http://127.0.0.1:") {
				return true
			}
			if strings.HasPrefix(origin, "https://localhost:") || strings.HasPrefix(origin, "https://127.0.0.1:") {
				return true
			}
			if isAllowedOrigin(internalConfig.App.FrontendDomain, origin) {
				return true
			}
			return false
		},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   append([]string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"}, supertokens.GetAllCORSHeaders()...),
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}

	router.Use(middlewares.RequestIDMiddleware)
	router.Use(middlewares.Logging(logger))
	router.Use(middlewares.BodyBuffer)
	router.Use(cors.Handler(corsOptions))
	router.Use(supertokens.Middleware)
	router.Use(middlewares.APIKeyAuth)
	router.Use(middlewares.SessionOptional)
	// router.Use(middlewares.Auth)

	// Conditional rate limiting based on authentication method
	normalLimiter, apiKeyLimiter := middlewares.CreateRateLimiters()
	router.Use(middlewares.ConditionalRateLimit(normalLimiter, apiKeyLimiter))

	router.Use(middlewares.ErrorHandler)

	// Registered last: all middlewares are already attached to the mux, so the
	// ordering invariant "middlewares before routes" holds.
	attachHealthRoute(router)

	endpointPrefix := fmt.Sprintf("/%s", internalConfig.App.EndpointPrefix)
	versionPrefix := fmt.Sprintf("/%s", internalConfig.App.Version)

	router.Route(endpointPrefix, func(r chi.Router) {
		r.Route(versionPrefix, func(r chi.Router) {
			r.Route("/auth", func(r chi.Router) {
				attachAuthRoutes(r, middlewares, authController)
			})

			attachPaymentRouter(r, middlewares, paymentController)
			attachScheduleRouter(r, middlewares, scheduleController)
			attachWebhookRouter(r, middlewares, webhookController)
			attachOrganizationRoutes(r, middlewares, organizationController)
			attachPrivacyRouter(r, purgeController)

			r.Mount("/tx", middlewares.TxProxy(internalConfig.FHIR.TerminologyServerBaseUrl))
		})
	})

	router.With(middlewares.Auth).
		Mount("/fhir", middlewares.Bridge(internalConfig.FHIR.BaseUrl))
}

// attachHealthRoute registers the /health liveness endpoint on the router.
// It must be called after every router.Use(...) on the same mux: chi requires
// all middlewares to be defined before routes, and a route cannot bypass the
// mux's own middleware stack.
func attachHealthRoute(router chi.Router) {
	router.Get("/health", handleHealth)
}

// handleHealth responds with the build metadata (version, git tag, commit
// hash) so production can verify the backend has been properly updated.
func handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":  "OK",
		"version": buildinfo.Version,
		"tag":     buildinfo.Tag,
		"hash":    buildinfo.CommitHash,
	})
}

func isAllowedOrigin(allowedDomain, origin string) bool {
	allowedDomain = strings.TrimSuffix(allowedDomain, "/")
	origin = strings.TrimSuffix(origin, "/")

	if allowedDomain == "" || origin == "" {
		return false
	}

	allowedURL, err := url.Parse(allowedDomain)
	if err != nil {
		allowedURL = &url.URL{Host: allowedDomain}
	}

	originURL, err := url.Parse(origin)
	if err != nil {
		return false
	}

	if strings.EqualFold(allowedURL.Hostname(), originURL.Hostname()) {
		return true
	}
	return false
}
