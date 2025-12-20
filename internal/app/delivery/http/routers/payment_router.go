package routers

import (
	"konsulin-service/internal/app/delivery/http/controllers"
	"konsulin-service/internal/app/delivery/http/middlewares"

	"github.com/go-chi/chi/v5"
)

func attachPaymentRouter(router chi.Router, middlewares *middlewares.Middlewares, paymentController *controllers.PaymentController) {
	router.Post("/pay/callback/xendit/invoice", paymentController.XenditInvoiceCallback)
	router.Post("/pay/service", paymentController.CreatePay)
	router.Post("/pay/appointment", paymentController.HandleAppointmentPayment)
}
