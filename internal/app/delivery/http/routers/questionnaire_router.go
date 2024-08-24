package routers

import (
	"konsulin-service/internal/app/delivery/http/middlewares"
	"konsulin-service/internal/app/services/core/questionnaires"

	"github.com/go-chi/chi/v5"
)

func attachQuestionnaireRouter(router chi.Router, middlewares *middlewares.Middlewares, questionnaireController *questionnaires.QuestionnaireController) {
	router.Post("/", questionnaireController.CreateQuestionnaire)
	router.Put("/{questionnaire_id}", questionnaireController.UpdateQuestionnaire)
	router.Get("/{questionnaire_id}", questionnaireController.FindQuestionnaireByID)
	router.Delete("/{questionnaire_id}", questionnaireController.DeleteQuestionnaireByID)
}
