package questionnaires

import (
	"context"
	"konsulin-service/internal/app/services/fhir_spark/questionnaires"
	fhir_dto "konsulin-service/internal/pkg/dto/fhir"
)

type questionnaireUsecase struct {
	QuestionnaireFhirClient questionnaires.QuestionnaireFhirClient
}

func NewQuestionnaireUsecase(
	questionnaireFhirClient questionnaires.QuestionnaireFhirClient,
) QuestionnaireUsecase {
	return &questionnaireUsecase{
		QuestionnaireFhirClient: questionnaireFhirClient,
	}
}

func (uc *questionnaireUsecase) CreateQuestionnaire(ctx context.Context, request *fhir_dto.Questionnaire) (*fhir_dto.Questionnaire, error) {
	questionnaire, err := uc.QuestionnaireFhirClient.CreateQuestionnaire(ctx, request)
	if err != nil {
		return nil, err
	}

	return questionnaire, nil
}
func (uc *questionnaireUsecase) UpdateQuestionnaire(ctx context.Context, request *fhir_dto.Questionnaire) (*fhir_dto.Questionnaire, error) {
	questionnaire, err := uc.QuestionnaireFhirClient.UpdateQuestionnaire(ctx, request)
	if err != nil {
		return nil, err
	}

	return questionnaire, nil
}

func (uc *questionnaireUsecase) FindQuestionnaireByID(ctx context.Context, questionnaireID string) (*fhir_dto.Questionnaire, error) {
	questionnaire, err := uc.QuestionnaireFhirClient.FindQuestionnaireByID(ctx, questionnaireID)
	if err != nil {
		return nil, err
	}

	return questionnaire, nil
}
func (uc *questionnaireUsecase) DeleteQuestionnaireByID(ctx context.Context, questionnaireID string) error {
	err := uc.QuestionnaireFhirClient.DeleteQuestionnaireByID(ctx, questionnaireID)
	if err != nil {
		return err
	}

	return nil
}
