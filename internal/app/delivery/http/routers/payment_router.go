package routers

import (
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/app/delivery/http/controllers"
	"konsulin-service/internal/app/delivery/http/middlewares"

	"github.com/go-chi/chi/v5"
)

// attachPaymentRouter registers the payment endpoints behind
// RequireGateway: standalone mode (nil or disabled gateway status) answers
// 503 with dev_message GATEWAY_NOT_CONFIGURED before the payment handler
// runs; an enabled and enrolled gateway delegates to the handler.
//
// Parameters: router is the chi subrouter the routes register on; mw holds
// the middlewares the guard lives on (unused directly: RequireGateway is a
// package-level middleware kept for signature symmetry with the sibling
// attach* helpers); paymentController serves both endpoints (nil tolerable:
// the guard rejects before it runs); status is the gateway status published
// on Controllers.Gateway, consulted per request.
// Returns: nothing; never panics.
func attachPaymentRouter(router chi.Router, mw *middlewares.Middlewares, paymentController *controllers.PaymentController, status contracts.GatewayStatus) {
	router.With(middlewares.RequireGateway(status)).
		Post("/pay/service", paymentController.CreatePay)
	router.With(middlewares.RequireGateway(status)).
		Post("/pay/appointment", paymentController.HandleAppointmentPayment)
}
