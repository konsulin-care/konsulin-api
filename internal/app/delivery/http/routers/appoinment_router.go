package routers

import (
	"konsulin-service/internal/app/delivery/http/controllers"
	"konsulin-service/internal/app/delivery/http/middlewares"

	"github.com/go-chi/chi/v5"
)

func attachAppointmentRoutes(router chi.Router, middlewares *middlewares.Middlewares, appointmentController *controllers.AppointmentController) {
	router.With(middlewares.Authenticate).Get("/", appointmentController.FindAll)
	router.With(middlewares.Authenticate).Post("/", appointmentController.CreateAppointment)
	router.With(middlewares.Authenticate).Post("/upcoming", appointmentController.UpcomingAppointment)
}
