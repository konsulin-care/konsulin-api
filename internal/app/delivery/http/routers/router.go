package routers

import (
	"fmt"
	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/delivery/http/controllers"
	"konsulin-service/internal/app/delivery/http/middlewares"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/go-chi/httprate"
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
			if origin == internalConfig.App.FrontendDomain {
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
	router.Use(cors.Handler(corsOptions))
	router.Use(supertokens.Middleware)
	router.Use(middlewares.SessionOptional)
	// router.Use(middlewares.Auth)

	// Rate limiting pake httprate
	rateLimiter := httprate.LimitByIP(internalConfig.App.MaxRequests, time.Second)
	router.Use(rateLimiter)

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
