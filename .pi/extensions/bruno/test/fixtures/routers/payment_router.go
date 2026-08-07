package routers

import (
	"github.com/go-chi/chi/v5"
	"konsulin-service/internal/app/delivery/http/controllers"
	"konsulin-service/internal/app/delivery/http/middlewares"
)

func attachPaymentRouter(router chi.Router, m *middlewares.Middlewares, paymentController *controllers.PaymentController) {
	router.Post("/pay/callback/xendit/invoice", paymentController.XenditInvoiceCallback)
	router.Post("/pay/service", paymentController.CreatePay)
	router.Post("/pay/appointment", paymentController.HandleAppointmentPayment)
}
