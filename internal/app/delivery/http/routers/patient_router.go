package routers

import (
	"konsulin-service/internal/app/delivery/http/middlewares"
	"konsulin-service/internal/app/services/core/patients"

	"github.com/go-chi/chi/v5"
)

func attachPatientRouter(router chi.Router, middlewares *middlewares.Middlewares, patientController *patients.PatientController) {
	router.With(middlewares.Authenticate).Post("/clinicians/{clinician_id}/appointments", patientController.CreateAppointment)
}
