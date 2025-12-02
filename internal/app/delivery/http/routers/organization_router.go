package routers

import (
	"konsulin-service/internal/app/delivery/http/controllers"
	"konsulin-service/internal/app/delivery/http/middlewares"

	"github.com/go-chi/chi/v5"
)

func attachOrganizationRoutes(router chi.Router, m *middlewares.Middlewares, c *controllers.OrganizationController) {
	router.Post("/organizations/{organizationId}/roles", c.RegisterPractitionerRole)
}


