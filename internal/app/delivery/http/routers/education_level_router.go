package routers

import (
	"konsulin-service/internal/app/delivery/http/controllers"
	"konsulin-service/internal/app/delivery/http/middlewares"

	"github.com/go-chi/chi/v5"
)

func attachEducationLevelRoutes(router chi.Router, middlewares *middlewares.Middlewares, educationLevelController *controllers.EducationLevelController) {
	router.Get("/", educationLevelController.FindAll)
}
