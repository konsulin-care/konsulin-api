package assessments

import (
	"context"
	"konsulin-service/internal/app/services/fhir_spark/questionnaires"
	"konsulin-service/internal/pkg/fhir_dto"
)

type assessmentUsecase struct {
	QuestionnaireFhirClient questionnaires.QuestionnaireFhirClient
}

func NewAssessmentUsecase(
	questionnaireFhirClient questionnaires.QuestionnaireFhirClient,
) AssessmentUsecase {
	return &assessmentUsecase{
		QuestionnaireFhirClient: questionnaireFhirClient,
	}
}

func (uc *assessmentUsecase) CreateAssessment(ctx context.Context, request *fhir_dto.Questionnaire) (*fhir_dto.Questionnaire, error) {
	questionnaire, err := uc.QuestionnaireFhirClient.CreateQuestionnaire(ctx, request)
	if err != nil {
		return nil, err
	}

	return questionnaire, nil
}
func (uc *assessmentUsecase) UpdateAssessment(ctx context.Context, request *fhir_dto.Questionnaire) (*fhir_dto.Questionnaire, error) {
	questionnaire, err := uc.QuestionnaireFhirClient.UpdateQuestionnaire(ctx, request)
	if err != nil {
		return nil, err
	}

	return questionnaire, nil
}

func (uc *assessmentUsecase) FindAssessmentByID(ctx context.Context, questionnaireID string) (*fhir_dto.Questionnaire, error) {
	questionnaire, err := uc.QuestionnaireFhirClient.FindQuestionnaireByID(ctx, questionnaireID)
	if err != nil {
		return nil, err
	}

	return questionnaire, nil
}
func (uc *assessmentUsecase) DeleteAssessmentByID(ctx context.Context, questionnaireID string) error {
	err := uc.QuestionnaireFhirClient.DeleteQuestionnaireByID(ctx, questionnaireID)
	if err != nil {
		return err
	}

	return nil
}
