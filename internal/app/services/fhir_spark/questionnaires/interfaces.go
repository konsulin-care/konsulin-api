package questionnaires

import (
	"context"
	"konsulin-service/internal/pkg/fhir_dto"
)

type QuestionnaireUsecase interface{}

type QuestionnaireRepository interface{}

type QuestionnaireFhirClient interface {
	CreateQuestionnaire(ctx context.Context, request *fhir_dto.Questionnaire) (*fhir_dto.Questionnaire, error)
	UpdateQuestionnaire(ctx context.Context, request *fhir_dto.Questionnaire) (*fhir_dto.Questionnaire, error)
	PatchQuestionnaire(ctx context.Context, request *fhir_dto.Questionnaire) (*fhir_dto.Questionnaire, error)
	FindQuestionnaireByID(ctx context.Context, questionnaireID string) (*fhir_dto.Questionnaire, error)
	DeleteQuestionnaireByID(ctx context.Context, questionnaireID string) error
}
