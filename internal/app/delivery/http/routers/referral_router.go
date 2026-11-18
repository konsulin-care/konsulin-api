package routers

import (
	"github.com/konsulin-care/konsulin-api/internal/app/contracts"
	"github.com/konsulin-care/konsulin-api/internal/app/delivery/http/controllers"
	"github.com/konsulin-care/konsulin-api/internal/app/delivery/http/middlewares"

	"github.com/go-chi/chi/v5"
)

// attachReferralRouter mounts the referral workflow, all behind
// RequireGateway (503 while the gateway is disabled or this party is not
// enrolled): POST /federation/referrals is the public ticket-verified
// inbound receive (the controller hands the bearer ticket to the usecase,
// which verifies it against the gateway approval plane before opening the
// sealed envelope), and the session routes POST /referrals,
// GET /referrals/incoming and POST /referrals/incoming/{id}/confirm|reject
// authorize per request with a session uid plus Casbin Enforce on the
// exact path (fail closed). A nil controller leaves every route
// unregistered so chi answers 404.
//
// Parameters: router is the chi subrouter the routes register on; mw
// supplies the Casbin enforcer (nil tolerable: authorization then fails
// closed); ctrls is the referral controller (nil tolerable: the routes
// stay unregistered); status is the gateway enrollment status
// RequireGateway consults per request.
// Returns: nothing; never panics.
func attachReferralRouter(router chi.Router, mw *middlewares.Middlewares, ctrls *controllers.ReferralController, status contracts.GatewayStatus) {
	if ctrls == nil {
		return
	}
	if mw != nil {
		ctrls.Enforcer = mw.Enforcer
	}
	r := router.With(middlewares.RequireGateway(status))
	r.Post("/federation/referrals", ctrls.Receive)
	r.Post("/referrals", ctrls.Send)
	r.Get("/referrals/incoming", ctrls.ListIncoming)
	r.Post("/referrals/incoming/{id}/confirm", ctrls.Confirm)
	r.Post("/referrals/incoming/{id}/reject", ctrls.Reject)
}
