package routers

import (
	"fmt"
	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/delivery/http/middlewares"
	"konsulin-service/internal/app/services/core/auth"
	"konsulin-service/internal/app/services/core/clinicians"
	"konsulin-service/internal/app/services/core/clinics"
	educationLevels "konsulin-service/internal/app/services/core/education_levels"
	"konsulin-service/internal/app/services/core/genders"
	"konsulin-service/internal/app/services/core/patients"
	questionnaireResponses "konsulin-service/internal/app/services/core/questionnaire_responses"
	"konsulin-service/internal/app/services/core/questionnaires"
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
	clinicController *clinics.ClinicController,
	clinicianController *clinicians.ClinicianController,
	patientController *patients.PatientController,
	educationLevelController *educationLevels.EducationLevelController,
	genderController *genders.GenderController,
	questionnaireController *questionnaires.QuestionnaireController,
	questionnaireResponseController *questionnaireResponses.QuestionnaireResponseController,
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

			r.Route("/clinics", func(r chi.Router) {
				attachClinicRoutes(r, middlewares, clinicController)
			})
			r.Route("/clinicians", func(r chi.Router) {
				attachClinicianRouter(r, middlewares, clinicianController)
			})
			r.Route("/patients", func(r chi.Router) {
				attachPatientRouter(r, middlewares, patientController)
			})
			r.Route("/questionnaire-responses", func(r chi.Router) {
				attachQuestionnaireResponseRouter(r, middlewares, questionnaireResponseController)
			})
			r.Route("/questionnaires", func(r chi.Router) {
				attachQuestionnaireRouter(r, middlewares, questionnaireController)
			})
		})
	})
}
