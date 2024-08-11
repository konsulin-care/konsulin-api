package routers

import (
	"konsulin-service/internal/app/delivery/http/middlewares"
	"konsulin-service/internal/app/services/core/clinicians"

	"github.com/go-chi/chi/v5"
)

func attachClinicianRouter(router chi.Router, middlewares *middlewares.Middlewares, clinicianController *clinicians.ClinicianController) {
	router.With(middlewares.Authenticate).Post("/clinics/availability", clinicianController.CreateClinicsAvailability)
	router.With(middlewares.Authenticate).Post("/clinics", clinicianController.CreateClinics)
	router.With(middlewares.Authenticate).Delete("/clinics/{clinic_id}", clinicianController.DeleteClinicByID)
}
