package routers

import (
	"konsulin-service/internal/app/delivery/http/controllers"
	"konsulin-service/internal/app/delivery/http/middlewares"

	"github.com/go-chi/chi/v5"
)

func attachGenderRoutes(router chi.Router, middlewares *middlewares.Middlewares, genderController *controllers.GenderController) {
	router.Get("/", genderController.FindAll)
}
