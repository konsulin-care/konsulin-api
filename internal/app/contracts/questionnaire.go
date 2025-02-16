package contracts

import (
	"context"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/fhir_dto"
)

type QuestionnaireFhirClient interface {
	FindQuestionnaires(ctx context.Context, request *requests.FindAllAssessment) ([]fhir_dto.Questionnaire, error)
	CreateQuestionnaire(ctx context.Context, request map[string]interface{}) (map[string]interface{}, error)
	UpdateQuestionnaire(ctx context.Context, request *fhir_dto.Questionnaire) (*fhir_dto.Questionnaire, error)
	PatchQuestionnaire(ctx context.Context, request *fhir_dto.Questionnaire) (*fhir_dto.Questionnaire, error)
	FindQuestionnaireByID(ctx context.Context, questionnaireID string) (*fhir_dto.Questionnaire, error)
	FindRawQuestionnaireByID(ctx context.Context, questionnaireID string) (map[string]interface{}, error)
	DeleteQuestionnaireByID(ctx context.Context, questionnaireID string) error
}
