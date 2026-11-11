package routers

import (
	"net/http"

	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/app/delivery/http/controllers"
	"konsulin-service/internal/app/delivery/http/middlewares"
	"konsulin-service/internal/app/drivers/gateway"

	"github.com/go-chi/chi/v5"
	"github.com/konsulin-care/gateway-sdk-go/wire"
	"go.uber.org/zap"
)

// attachFederationRouter mounts the party's inbound federation callbacks.
// POST /federation/events (the party's endpoints.callbacks) is guarded by
// RequireGateway (503 while the gateway is disabled or this party is not
// enrolled) and the T20 VerifyMiddleware for ks-event+jwt (gateway-signed
// token, JWKS verification, webhook-id dedupe), then handed to the
// FederationController. POST /federation/deliveries/{listing} is guarded the
// same way with the ks-delivery+jwt VerifyMiddleware and handed to
// FederationController.HandleDelivery, which enforces that the verified
// service claim equals the {listing} path segment (401) and that the
// listing is registered (404). Both sit on the mux chain — session and RBAC
// are not part of federation callbacks.
//
// A VerifyMiddleware construction failure (empty APP_GATEWAY_URL or
// APP_PARTY_ID) cannot be returned from SetupRoutes, so each route fails
// closed: every callback is denied 401 instead of reaching the handler
// unverified.
//
// Parameters: router is the chi subrouter the routes register on; ctrls is
// the federation controller (nil tolerable: the handler only runs when a
// request matches); status is the gateway enrollment status RequireGateway
// consults per request; logger records construction failures.
// Returns: nothing; never panics.
func attachFederationRouter(router chi.Router, ctrls *controllers.FederationController, status contracts.GatewayStatus, logger *zap.Logger) {
	verifyEvents, err := gateway.VerifyMiddleware(wire.TypEvent)
	if err != nil {
		if logger != nil {
			logger.Warn("federation: event verification unavailable, callbacks denied",
				zap.Error(err))
		}
		verifyEvents = denyUnverified
	}
	router.With(middlewares.RequireGateway(status), verifyEvents).
		Post("/federation/events", ctrls.HandleEvent)

	verifyDeliveries, err := gateway.VerifyMiddleware(wire.TypDelivery)
	if err != nil {
		if logger != nil {
			logger.Warn("federation: delivery verification unavailable, callbacks denied",
				zap.Error(err))
		}
		verifyDeliveries = denyUnverified
	}
	router.With(middlewares.RequireGateway(status), verifyDeliveries).
		Post("/federation/deliveries/{listing}", ctrls.HandleDelivery)
}

// denyUnverified is the fail-closed stand-in for an unbuildable
// VerifyMiddleware: it answers 401 to every callback without invoking the
// next handler, so a request can never reach the federation controller
// without gateway verification.
//
// Parameters: next is the handler that must not run.
// Returns: a handler that denies 401 unconditionally.
func denyUnverified(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
	})
}
