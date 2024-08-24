package questionnaires

import (
	"context"
	fhir_dto "konsulin-service/internal/pkg/dto/fhir"
)

type QuestionnaireUsecase interface {
	CreateQuestionnaire(ctx context.Context, request *fhir_dto.Questionnaire) (*fhir_dto.Questionnaire, error)
	UpdateQuestionnaire(ctx context.Context, request *fhir_dto.Questionnaire) (*fhir_dto.Questionnaire, error)
	FindQuestionnaireByID(ctx context.Context, questionnaireID string) (*fhir_dto.Questionnaire, error)
	DeleteQuestionnaireByID(ctx context.Context, questionnaireID string) error
}

type QuestionnaireRepository interface{}
