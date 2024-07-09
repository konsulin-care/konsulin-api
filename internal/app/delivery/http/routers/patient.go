package routers

import (
	"konsulin-service/internal/app/delivery/http/middlewares"
	"konsulin-service/internal/app/services/patients"

	"github.com/go-chi/chi/v5"
)

func attachPatientRoutes(router chi.Router, middlewares *middlewares.Middlewares, patientController *patients.PatientController) {
	router.With(middlewares.AuthMiddleware).Get("/me", patientController.GetPatientProfileBySession)
	router.With(middlewares.AuthMiddleware).Put("/me", patientController.UpdateProfileBySession)
}
