package routers

import (
	"konsulin-service/internal/app/delivery/http/middlewares"
	"konsulin-service/internal/app/services/core/assessments"

	"github.com/go-chi/chi/v5"
)

func attachQuestionnaireRouter(router chi.Router, middlewares *middlewares.Middlewares, assessmentController *assessments.AssessmentController) {
	router.Post("/", assessmentController.CreateAssessment)
	router.Put("/{assessment_id}", assessmentController.UpdateAssessment)
	router.Get("/{assessment_id}", assessmentController.FindAssessmentByID)
	router.Delete("/{assessment_id}", assessmentController.DeleteAssessmentByID)
}
