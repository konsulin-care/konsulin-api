package assessments

import (
	"context"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/pkg/dto/responses"
	"konsulin-service/internal/pkg/fhir_dto"
)

type assessmentUsecase struct {
	QuestionnaireFhirClient contracts.QuestionnaireFhirClient
}

func NewAssessmentUsecase(
	questionnaireFhirClient contracts.QuestionnaireFhirClient,
) contracts.AssessmentUsecase {
	return &assessmentUsecase{
		QuestionnaireFhirClient: questionnaireFhirClient,
	}
}

func (uc *assessmentUsecase) FindAll(ctx context.Context) ([]responses.Assessment, error) {
	questionnaires, err := uc.QuestionnaireFhirClient.FindQuestionnaires(ctx)
	if err != nil {
		return nil, err
	}

	assessments := uc.mapFHIRQuestionnaireToAssessment(questionnaires)
	return assessments, nil
}

func (uc *assessmentUsecase) mapFHIRQuestionnaireToAssessment(questionnaires []fhir_dto.Questionnaire) []responses.Assessment {
	assessments := make([]responses.Assessment, 0, len(questionnaires))
	for _, eachQuestionnaire := range questionnaires {
		assessment := responses.Assessment{
			AssessmentID: eachQuestionnaire.ID,
			Title:        eachQuestionnaire.Title,
		}
		assessments = append(assessments, assessment)
	}

	return assessments
}

func (uc *assessmentUsecase) CreateAssessment(ctx context.Context, request map[string]interface{}) (map[string]interface{}, error) {
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

func (uc *assessmentUsecase) FindAssessmentByID(ctx context.Context, questionnaireID string) (map[string]interface{}, error) {
	questionnaire, err := uc.QuestionnaireFhirClient.FindRawQuestionnaireByID(ctx, questionnaireID)
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
