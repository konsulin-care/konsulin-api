package routers

import (
	"github.com/konsulin-care/konsulin-api/internal/app/delivery/http/controllers"
	"github.com/konsulin-care/konsulin-api/internal/app/delivery/http/middlewares"

	"github.com/go-chi/chi/v5"
)

func attachOrganizationRoutes(router chi.Router, m *middlewares.Middlewares, c *controllers.OrganizationController) {
	router.Post("/organizations/{organizationId}/roles", c.RegisterPractitionerRole)
}
