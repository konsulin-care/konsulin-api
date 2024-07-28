package routers

import (
	"fmt"
	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/delivery/http/middlewares"
	"konsulin-service/internal/app/services/core/auth"
	educationLevels "konsulin-service/internal/app/services/core/education_levels"
	"konsulin-service/internal/app/services/core/genders"
	"konsulin-service/internal/app/services/core/users"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/go-chi/httprate"
)

func SetupRoutes(
	router *chi.Mux,
	internalConfig *config.InternalConfig,
	middlewares *middlewares.Middlewares,
	userController *users.UserController,
	authController *auth.AuthController,
	educationLevelController *educationLevels.EducationLevelController,
	genderController *genders.GenderController,
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

	router.Use(middlewares.ErrorHandler)

	endpointPrefix := fmt.Sprintf("/%s", internalConfig.App.EndpointPrefix)
	versionPrefix := fmt.Sprintf("/%s", internalConfig.App.Version)

	router.Route(endpointPrefix, func(r chi.Router) {
		r.Route(versionPrefix, func(r chi.Router) {
			r.Route("/auth", func(r chi.Router) {
				attachAuthRoutes(r, middlewares, authController)
			})

			r.Route("/users", func(r chi.Router) {
				attachUserRoutes(r, middlewares, userController)
			})

			r.Route("/education-levels", func(r chi.Router) {
				attachEducationLevelRoutes(r, middlewares, educationLevelController)
			})

			r.Route("/genders", func(r chi.Router) {
				attachGenderRoutes(r, middlewares, genderController)
			})
		})
	})
}
