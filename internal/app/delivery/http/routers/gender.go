package routers

import (
	"konsulin-service/internal/app/delivery/http/middlewares"
	"konsulin-service/internal/app/services/core/genders"

	"github.com/go-chi/chi/v5"
)

func attachGenderRoutes(router chi.Router, middlewares *middlewares.Middlewares, genderController *genders.GenderController) {
	router.Get("/", genderController.FindAll)
}
