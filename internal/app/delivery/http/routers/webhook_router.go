package routers

import (
	"konsulin-service/internal/app/delivery/http/controllers"
	"konsulin-service/internal/app/delivery/http/middlewares"

	"github.com/go-chi/chi/v5"
)

func attachWebhookRouter(router chi.Router, middlewares *middlewares.Middlewares, ctrl *controllers.WebhookController) {
	// POST /hook/{service}
	router.Post("/hook/{service}", ctrl.HandleEnqueueWebHook)
}


