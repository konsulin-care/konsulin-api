package routers

import (
	"fmt"
	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/delivery/http/controllers"
	"konsulin-service/internal/app/delivery/http/middlewares"
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
) {
	corsOptions := cors.Options{
		AllowOriginFunc: func(r *http.Request, origin string) bool {
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
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
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

	endpointPrefix := fmt.Sprintf("/%s", internalConfig.App.EndpointPrefix)
	versionPrefix := fmt.Sprintf("/%s", internalConfig.App.Version)

	router.Route(endpointPrefix, func(r chi.Router) {
		r.Route(versionPrefix, func(r chi.Router) {
			r.Route("/auth", func(r chi.Router) {
				attachAuthRoutes(r, middlewares, authController)
			})
		})
	})

	router.With(middlewares.Auth).
		Mount("/fhir", middlewares.Bridge(internalConfig.FHIR.BaseUrl))
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
