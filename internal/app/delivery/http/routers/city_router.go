package routers

import (
	"konsulin-service/internal/app/delivery/http/controllers"
	"konsulin-service/internal/app/delivery/http/middlewares"

	"github.com/go-chi/chi/v5"
)

func attachCityRoutes(router chi.Router, middlewares *middlewares.Middlewares, cityController *controllers.CityController) {
	router.Get("/", cityController.FindAll)
}
