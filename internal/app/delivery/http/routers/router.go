package routers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/app/delivery/http/controllers"
	"konsulin-service/internal/app/delivery/http/middlewares"
	"konsulin-service/internal/pkg/buildinfo"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/supertokens/supertokens-golang/supertokens"
	"go.uber.org/zap"
)

// Controllers bundles everything SetupRoutes needs beyond the router,
// config, logger, and middlewares. The zero value is valid: nil controller
// pointers are tolerated at registration time (route handlers only run when
// a request matches), and later tasks can add fields without touching
// existing call sites.
type Controllers struct {
	Auth         *controllers.AuthController
	Payment      *controllers.PaymentController
	Schedule     *controllers.ScheduleController
	Organization *controllers.OrganizationController
	Purge        *controllers.PurgeController
	Federation   *controllers.FederationController
	Services     *controllers.ServicesController
	Referral     *controllers.ReferralController
	Research     *controllers.ResearchController
	Gateway      contracts.GatewayStatus
}

// registeredResearch holds the research controller published by
// cmd/http's registerResearch during bootstrap: main.go does not pass
// ctrls into that function, so the controller reaches SetupRoutes through
// this variable when Controllers.Research is unset. Zero value nil:
// attachResearchRouter then registers nothing and chi answers 404.
var registeredResearch *controllers.ResearchController

// RegisterResearch publishes the research controller for SetupRoutes to
// mount when Controllers.Research is nil. Call it during bootstrap, before
// SetupRoutes.
// Parameters: ctrl is the controller to mount (nil clears the fallback).
// Returns nothing; never panics.
func RegisterResearch(ctrl *controllers.ResearchController) {
	registeredResearch = ctrl
}

// SetupRoutes wires the middleware chain and registers every route on the
// given mux. Controllers is passed as a struct so new dependencies can be
// added without changing the parameter list.
func SetupRoutes(
	router *chi.Mux,
	internalConfig *config.InternalConfig,
	logger *zap.Logger,
	middlewares *middlewares.Middlewares,
	ctrls Controllers,
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
				attachAuthRoutes(r, middlewares, ctrls.Auth)
			})

			attachPaymentRouter(r, middlewares, ctrls.Payment, ctrls.Gateway)
			attachScheduleRouter(r, middlewares, ctrls.Schedule)
			// T50: the legacy WebhookController is gone; the temporary /hook
			// aliases bind without any controller parameter.
			attachWebhookRouter(r, middlewares)
			attachOrganizationRoutes(r, middlewares, ctrls.Organization)
			attachPrivacyRouter(r, ctrls.Purge)
			attachFederationRouter(r, ctrls.Federation, ctrls.Gateway, logger)
			attachServicesRouter(r, middlewares, ctrls.Services, ctrls.Gateway)
			attachReferralRouter(r, middlewares, ctrls.Referral, ctrls.Gateway)
			research := ctrls.Research
			if research == nil {
				research = registeredResearch
			}
			attachResearchRouter(r, research, ctrls.Gateway)
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
