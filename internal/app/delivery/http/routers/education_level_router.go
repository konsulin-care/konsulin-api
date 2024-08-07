package routers

import (
	"konsulin-service/internal/app/delivery/http/middlewares"
	educationLevels "konsulin-service/internal/app/services/core/education_levels"

	"github.com/go-chi/chi/v5"
)

func attachEducationLevelRoutes(router chi.Router, middlewares *middlewares.Middlewares, educationLevelController *educationLevels.EducationLevelController) {
	router.Get("/", educationLevelController.FindAll)
}
