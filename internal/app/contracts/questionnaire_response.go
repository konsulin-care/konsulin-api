package contracts

import (
	"context"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/fhir_dto"
)

type QuestionnaireResponseFhirClient interface {
	FindQuestionnaireResponses(ctx context.Context, request *requests.FindAllAssessmentResponse) ([]fhir_dto.QuestionnaireResponse, error)
	CreateQuestionnaireResponse(ctx context.Context, request map[string]interface{}) (map[string]interface{}, error)
	UpdateQuestionnaireResponse(ctx context.Context, request *fhir_dto.QuestionnaireResponse) (*fhir_dto.QuestionnaireResponse, error)
	PatchQuestionnaireResponse(ctx context.Context, request *fhir_dto.QuestionnaireResponse) (*fhir_dto.QuestionnaireResponse, error)
	FindQuestionnaireResponseByID(ctx context.Context, questionnaireResponseID string) (*fhir_dto.QuestionnaireResponse, error)
	DeleteQuestionnaireResponseByID(ctx context.Context, questionnaireResponseID string) error
}
