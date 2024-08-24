package routers

import (
	"konsulin-service/internal/app/delivery/http/middlewares"
	questionnaireResponses "konsulin-service/internal/app/services/core/questionnaire_responses"

	"github.com/go-chi/chi/v5"
)

func attachQuestionnaireResponseRouter(router chi.Router, middlewares *middlewares.Middlewares, questionnaireResponseController *questionnaireResponses.QuestionnaireResponseController) {
	router.Post("/", questionnaireResponseController.CreateQuestionnaireResponse)
	router.Put("/{questionnaire_response_id}", questionnaireResponseController.UpdateQuestionnaireResponse)
	router.Get("/{questionnaire_response_id}", questionnaireResponseController.FindQuestionnaireResponseByID)
	router.Delete("/{questionnaire_response_id}", questionnaireResponseController.DeleteQuestionnaireResponseByID)
}
