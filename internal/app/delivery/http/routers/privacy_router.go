package routers

import (
	"konsulin-service/internal/app/delivery/http/controllers"
	"konsulin-service/internal/app/delivery/http/middlewares"

	"github.com/go-chi/chi/v5"
)

// attachPrivacyRouter registers the erasure endpoint. Session identity is
// resolved inside the controller the same way the Auth middleware resolves it.
func attachPrivacyRouter(router chi.Router, m *middlewares.Middlewares, c *controllers.PurgeController) {
	router.Delete("/privacy/purge", c.PurgeData)
}
