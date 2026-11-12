package routers

import (
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/app/delivery/http/controllers"
	"konsulin-service/internal/app/delivery/http/middlewares"

	"github.com/go-chi/chi/v5"
)

// attachServicesRouter mounts the marketplace service endpoints, all
// behind RequireGateway (503 while the gateway is disabled or this party
// is not enrolled): GET /services lists the T30 cached webhook listings
// filtered by Casbin Enforce for the caller's roles on each listing's
// POST path, and POST /services/{provider}/{name} resolves the provider,
// authorizes the exact path, applies the per-user limiter and queues an
// async invocation through the services usecase. The T36B sync route
// POST /services/{provider}/{name}/sync answers an authorized inline
// invocation with its opened reply, and GET /services/requests/{id} serves
// the owner-only {status, result} view. The controller's
// Enforcer is wired from the middleware stack here so the handlers
// consult the same RBAC enforcer as the rest of the API.
//
// Parameters: router is the chi subrouter the routes register on; mw
// supplies the Casbin enforcer (nil tolerable: authorization then fails
// closed); ctrls is the services controller (nil tolerable: the routes
// stay unregistered and chi answers 404); status is the gateway
// enrollment status RequireGateway consults per request.
// Returns: nothing; never panics.
func attachServicesRouter(router chi.Router, mw *middlewares.Middlewares, ctrls *controllers.ServicesController, status contracts.GatewayStatus) {
	if ctrls == nil {
		return
	}
	if mw != nil {
		ctrls.Enforcer = mw.Enforcer
	}
	r := router.With(middlewares.RequireGateway(status))
	r.Get("/services", ctrls.ListServices)
	r.Post("/services/{provider}/{name}", ctrls.InvokeService)
	r.Post("/services/{provider}/{name}/sync", ctrls.SyncService)
	r.Get("/services/requests/{id}", ctrls.GetServiceRequest)
}
