package routers

import (
	"konsulin-service/internal/app/delivery/http/controllers"
	"konsulin-service/internal/app/delivery/http/middlewares"

	"github.com/go-chi/chi/v5"
)

func attachWebhookRouter(router chi.Router, m *middlewares.Middlewares, ctrl *controllers.WebhookController) {
	router.Post("/hook/synchronous/{service}", ctrl.HandleSynchronousWebHook)
	router.Post("/hook/{service}", ctrl.HandleEnqueueWebHook)
	router.Post("/callback/service-request", ctrl.HandleAsyncServiceResultCallback)
	router.Get("/service-request/{id}/result", ctrl.HandleGetAsyncServiceResult)
}
