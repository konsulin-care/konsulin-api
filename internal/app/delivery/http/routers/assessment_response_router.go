package routers

import (
	"konsulin-service/internal/app/delivery/http/controllers"
	"konsulin-service/internal/app/delivery/http/middlewares"

	"github.com/go-chi/chi/v5"
)

func attachQuestionnaireResponseRouter(router chi.Router, middlewares *middlewares.Middlewares, assessmentResponseController *controllers.AssessmentResponseController) {
	router.With(middlewares.Authenticate).Get("/", assessmentResponseController.FindAll)
	router.With(middlewares.OptionalAuthenticate).Post("/", assessmentResponseController.CreateAssesmentResponse)
	router.Put("/{assessment_response_id}", assessmentResponseController.UpdateAssessmentResponse)
	router.Get("/{assessment_response_id}", assessmentResponseController.FindQuestionnaireResponseByID)
	router.Delete("/{assessment_response_id}", assessmentResponseController.DeleteQuestionnaireResponseByID)
}
