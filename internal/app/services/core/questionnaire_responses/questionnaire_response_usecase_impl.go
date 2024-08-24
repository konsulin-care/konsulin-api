package questionnaireResponses

import (
	"context"
	questionnaireResponses "konsulin-service/internal/app/services/fhir_spark/questionnaires_responses"
	fhir_dto "konsulin-service/internal/pkg/dto/fhir"
)

type questionnaireResponseUsecase struct {
	QuestionnaireResponseFhirClient questionnaireResponses.QuestionnaireResponseFhirClient
}

func NewQuestionnaireResponseUsecase(
	questionnaireResponseFhirClient questionnaireResponses.QuestionnaireResponseFhirClient,
) QuestionnaireResponseUsecase {
	return &questionnaireResponseUsecase{
		QuestionnaireResponseFhirClient: questionnaireResponseFhirClient,
	}
}

func (uc *questionnaireResponseUsecase) CreateQuestionnaireResponse(ctx context.Context, request *fhir_dto.QuestionnaireResponse) (*fhir_dto.QuestionnaireResponse, error) {
	questionnaireResponse, err := uc.QuestionnaireResponseFhirClient.CreateQuestionnaireResponse(ctx, request)
	if err != nil {
		return nil, err
	}

	return questionnaireResponse, nil
}
func (uc *questionnaireResponseUsecase) UpdateQuestionnaireResponse(ctx context.Context, request *fhir_dto.QuestionnaireResponse) (*fhir_dto.QuestionnaireResponse, error) {
	questionnaireResponse, err := uc.QuestionnaireResponseFhirClient.UpdateQuestionnaireResponse(ctx, request)
	if err != nil {
		return nil, err
	}

	return questionnaireResponse, nil
}

func (uc *questionnaireResponseUsecase) FindQuestionnaireResponseByID(ctx context.Context, questionnaireResponseID string) (*fhir_dto.QuestionnaireResponse, error) {
	questionnaireResponse, err := uc.QuestionnaireResponseFhirClient.FindQuestionnaireResponseByID(ctx, questionnaireResponseID)
	if err != nil {
		return nil, err
	}

	return questionnaireResponse, nil
}
func (uc *questionnaireResponseUsecase) DeleteQuestionnaireResponseByID(ctx context.Context, questionnaireResponseID string) error {
	err := uc.QuestionnaireResponseFhirClient.DeleteQuestionnaireResponseByID(ctx, questionnaireResponseID)
	if err != nil {
		return err
	}

	return nil
}
