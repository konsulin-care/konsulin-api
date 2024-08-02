package routers

import (
	"konsulin-service/internal/app/delivery/http/middlewares"
	"konsulin-service/internal/app/services/core/clinics"

	"github.com/go-chi/chi/v5"
)

func attachClinicRoutes(router chi.Router, middlewares *middlewares.Middlewares, clinicController *clinics.ClinicController) {
	router.With(middlewares.Authenticate).Get("/", clinicController.FindAll)

}
