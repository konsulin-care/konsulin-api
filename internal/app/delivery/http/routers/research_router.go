package routers

import (
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/app/delivery/http/controllers"
	"konsulin-service/internal/app/delivery/http/middlewares"

	"github.com/go-chi/chi/v5"
)

// attachResearchRouter mounts the research databank HTTP surface.
// POST /research/studies/{id}/withdraw registers on the plain router (the
// session chain already carries the caller uid and roles, and the handler
// enforces Patient plus fail-closed collaborators itself), while the
// researcher and cross-party export routes sit behind RequireGateway, so
// standalone mode (gateway disabled or party not enrolled) answers 503
// before any handler runs. Role checks (Patient, Researcher) live in the
// handlers. A nil controller leaves every route unregistered so chi
// answers 404.
//
// Parameters: router is the chi subrouter the routes register on; ctrls
// is the research controller (nil tolerable: the routes stay
// unregistered); status is the gateway enrollment status RequireGateway
// consults per request.
// Returns: nothing; never panics.
func attachResearchRouter(router chi.Router, ctrls *controllers.ResearchController, status contracts.GatewayStatus) {
	if ctrls == nil {
		return
	}
	router.Post("/research/studies/{id}/withdraw", ctrls.WithdrawStudy)
	r := router.With(middlewares.RequireGateway(status))
	r.Post("/research/requests", ctrls.RequestExport)
	r.Get("/research/requests", ctrls.ListRequests)
	r.Post("/federation/research/exports", ctrls.ReceiveExport)
	r.Get("/federation/research/exports/downloads/{token}", ctrls.DownloadExport)
}
