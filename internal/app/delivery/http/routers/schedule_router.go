package routers

import (
	"konsulin-service/internal/app/delivery/http/controllers"
	"konsulin-service/internal/app/delivery/http/middlewares"

	"github.com/go-chi/chi/v5"
)

func attachScheduleRouter(router chi.Router, m *middlewares.Middlewares, c *controllers.ScheduleController) {
	router.Post("/schedule/unavailable", c.SetUnavailable)
}
