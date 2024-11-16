package routers

import (
	"fmt"
	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/delivery/http/controllers"
	"konsulin-service/internal/app/delivery/http/middlewares"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/go-chi/httprate"
)

func SetupRoutes(
	router *chi.Mux,
	internalConfig *config.InternalConfig,
	middlewares *middlewares.Middlewares,
	userController *controllers.UserController,
	authController *controllers.AuthController,
	clinicController *controllers.ClinicController,
	clinicianController *controllers.ClinicianController,
	patientController *controllers.PatientController,
	educationLevelController *controllers.EducationLevelController,
	cityController *controllers.CityController,
	genderController *controllers.GenderController,
	assessmentController *controllers.AssessmentController,
	assessmentResponseController *controllers.AssessmentResponseController,
	appointmentController *controllers.AppointmentController,
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

			r.Route("/cities", func(r chi.Router) {
				attachCityRoutes(r, middlewares, cityController)
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
			r.Route("/assessment-responses", func(r chi.Router) {
				attachQuestionnaireResponseRouter(r, middlewares, assessmentResponseController)
			})
			r.Route("/assessments", func(r chi.Router) {
				attachQuestionnaireRouter(r, middlewares, assessmentController)
			})
			r.Route("/appointments", func(r chi.Router) {
				attachAppointmentRoutes(r, middlewares, appointmentController)
			})
		})
	})
}
