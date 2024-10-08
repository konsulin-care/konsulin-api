package routers

import (
	"konsulin-service/internal/app/delivery/http/controllers"
	"konsulin-service/internal/app/delivery/http/middlewares"

	"github.com/go-chi/chi/v5"
)

func attachClinicianRouter(router chi.Router, middlewares *middlewares.Middlewares, clinicianController *controllers.ClinicianController) {
	router.With(middlewares.Authenticate).Post("/clinics/practice-availability", clinicianController.CreatePracticeAvailability)
	router.With(middlewares.Authenticate).Post("/clinics/practice-information", clinicianController.CreatePracticeInformation)
	router.With(middlewares.Authenticate).Get("/{clinician_id}/clinics", clinicianController.FindClinicsByClinicianID)
	router.With(middlewares.Authenticate).Get("/availability", clinicianController.FindAvailability)
	router.With(middlewares.Authenticate).Delete("/clinics/{clinic_id}", clinicianController.DeleteClinicByID)
}
