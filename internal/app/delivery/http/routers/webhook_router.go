package routers

import (
	"github.com/konsulin-care/konsulin-api/internal/app/delivery/http/controllers"
	"github.com/konsulin-care/konsulin-api/internal/app/delivery/http/middlewares"

	"github.com/go-chi/chi/v5"
)

// attachWebhookRouter binds the temporary T38 /hook aliases to the
// ServicesController published by controllers.NewServicesController:
// POST /hook/interpret runs the async automation/interpret invocation,
// POST /hook/synchronous/{service} runs the inline invocation for the
// allowlisted automation listings, POST /hook/{service} fails closed 404
// for every other name, and GET /service-request/{id}/result serves the
// owner-only result view. POST /callback/service-request and the legacy
// WebhookController bindings (and the controller parameter itself) are
// gone: this file's purpose is now solely the temporary /hook aliases. The
// alias routes are not wrapped in RequireGateway (this call site carries no
// gateway status); the handlers fail closed on their own when a gateway
// seam is missing.
//
// Parameters: router is the chi subrouter the routes register on; mw
// supplies the Casbin enforcer wired onto the controller (nil tolerable:
// authorization then fails closed). Returns: nothing; never panics.
func attachWebhookRouter(router chi.Router, middlewares *middlewares.Middlewares) {
	svc := controllers.HookAliasController()
	if svc == nil {
		// No ServicesController constructed (zero-value wiring): the alias
		// routes stay unregistered and chi answers 404 for every /hook path.
		return
	}
	if middlewares != nil {
		svc.Enforcer = middlewares.Enforcer
	}

	// POST /hook/interpret -> async automation/interpret
	router.Post("/hook/interpret", svc.HookInterpret)

	// POST /hook/synchronous/{modify-profile|update-avatar|send-wa-link}
	// -> sync automation listing; any other name fails closed 404.
	router.Post("/hook/synchronous/{service}", svc.HookSynchronous)

	// POST /hook/{service} -> fail-closed 404 for every unmapped name.
	router.Post("/hook/{service}", svc.HookUnknown)

	// GET /service-request/{id}/result -> owner-only result view.
	router.Get("/service-request/{id}/result", svc.GetServiceRequest)
}
