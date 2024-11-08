package routers

import (
	"konsulin-service/internal/app/delivery/http/controllers"
	"konsulin-service/internal/app/delivery/http/middlewares"

	"github.com/go-chi/chi/v5"
)

func attachClinicRoutes(router chi.Router, middlewares *middlewares.Middlewares, clinicController *controllers.ClinicController) {
	router.Get("/", clinicController.FindAll)
	router.Get("/{clinic_id}", clinicController.FindByID)
	router.Get("/{clinic_id}/clinicians", clinicController.FindAllCliniciansByClinicID)
	router.Get("/{clinic_id}/clinicians/{clinician_id}", clinicController.FindClinicianByClinicAndClinicianID)
}
