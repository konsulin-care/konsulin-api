package routers

import (
	"konsulin-service/internal/app/delivery/http/controllers"
	"konsulin-service/internal/app/delivery/http/middlewares"

	"github.com/go-chi/chi/v5"
)

func attachPaymentRouter(router chi.Router, middlewares *middlewares.Middlewares, paymentController *controllers.PaymentController) {
	router.Post("/payment-routing/callback", paymentController.PaymentRoutingCallback)
	router.Post("/pay", paymentController.CreatePay)
}
