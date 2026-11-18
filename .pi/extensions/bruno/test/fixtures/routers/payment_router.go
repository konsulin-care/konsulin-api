package routers

import (
	"github.com/konsulin-care/konsulin-api/internal/app/delivery/http/controllers"
	"github.com/konsulin-care/konsulin-api/internal/app/delivery/http/middlewares"

	"github.com/go-chi/chi/v5"
)

func attachPaymentRouter(router chi.Router, m *middlewares.Middlewares, paymentController *controllers.PaymentController) {
	router.Post("/pay/callback/xendit/invoice", paymentController.XenditInvoiceCallback)
	router.Post("/pay/service", paymentController.CreatePay)
	router.Post("/pay/appointment", paymentController.HandleAppointmentPayment)
}
