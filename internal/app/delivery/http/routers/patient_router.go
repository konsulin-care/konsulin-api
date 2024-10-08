package routers

import (
	"konsulin-service/internal/app/delivery/http/controllers"
	"konsulin-service/internal/app/delivery/http/middlewares"

	"github.com/go-chi/chi/v5"
)

func attachPatientRouter(router chi.Router, middlewares *middlewares.Middlewares, patientController *controllers.PatientController) {
	router.With(middlewares.Authenticate).Post("/clinicians/{clinician_id}/appointments", patientController.CreateAppointment)
}
