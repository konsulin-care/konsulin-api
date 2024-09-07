package routers

import (
	"konsulin-service/internal/app/delivery/http/middlewares"
	assessmentResponses "konsulin-service/internal/app/services/core/assessment_responses"

	"github.com/go-chi/chi/v5"
)

func attachQuestionnaireResponseRouter(router chi.Router, middlewares *middlewares.Middlewares, assessmentResponseController *assessmentResponses.AssessmentResponseController) {
	router.Post("/", assessmentResponseController.CreateAssesmentResponse)
	router.Put("/{assessment_response_id}", assessmentResponseController.UpdateAssessmentResponse)
	router.Get("/{assessment_response_id}", assessmentResponseController.FindQuestionnaireResponseByID)
	router.Delete("/{assessment_response_id}", assessmentResponseController.DeleteQuestionnaireResponseByID)
}
