package routers

import (
	"konsulin-service/internal/app/delivery/http/controllers"
	"konsulin-service/internal/app/delivery/http/middlewares"

	"github.com/go-chi/chi/v5"
)

func attachQuestionnaireRouter(router chi.Router, middlewares *middlewares.Middlewares, assessmentController *controllers.AssessmentController) {
	router.With(middlewares.OptionalAuthenticate).Get("/", assessmentController.FindAll)
	router.Post("/", assessmentController.CreateAssessment)
	router.Put("/{assessment_id}", assessmentController.UpdateAssessment)
	router.Get("/{assessment_id}", assessmentController.FindAssessmentByID)
	router.Delete("/{assessment_id}", assessmentController.DeleteAssessmentByID)
}
