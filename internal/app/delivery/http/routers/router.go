package routers

import (
	"fmt"
	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/delivery/http/middlewares"
	"konsulin-service/internal/app/services/auth"
	"konsulin-service/internal/app/services/patients"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/go-chi/httprate"
	"github.com/sirupsen/logrus"
)

func SetupRoutes(
	router *chi.Mux,
	internalConfig *config.InternalConfig,
	log *logrus.Logger,
	middlewares *middlewares.Middlewares,
	patientController *patients.PatientController,
	authController *auth.AuthController,
) {

	corsOptions := cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}
	router.Use(cors.Handler(corsOptions))

	// Rate limiting middleware using httprate
	rateLimiter := httprate.LimitByIP(internalConfig.App.MaxRequests, time.Second)
	router.Use(rateLimiter)

	// Logging middleware
	// router.Use(middlewares.RequestLogger(internalConfig.App, log))
	router.Use(middlewares.ErrorHandler)

	endpointPrefix := fmt.Sprintf("/%s", internalConfig.App.EndpointPrefix)
	versionPrefix := fmt.Sprintf("/%s", internalConfig.App.Version)

	// router := chi.NewRouter()
	router.Route(endpointPrefix, func(r chi.Router) {
		r.Route(versionPrefix, func(r chi.Router) {
			r.Route("/auth", func(r chi.Router) {
				attachAuthRoutes(r, middlewares, authController)
			})

			r.Route("/patients", func(r chi.Router) {
				attachPatientRoutes(r, middlewares, patientController)
			})
		})
	})
}
