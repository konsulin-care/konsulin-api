package questionnaireResponses

import (
	"context"
	fhir_dto "konsulin-service/internal/pkg/dto/fhir"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/dto/responses"
)

type QuestionnaireResponseUsecase interface {
	CreateQuestionnaireResponse(ctx context.Context, request *requests.CreateQuestionnaireResponse) (*responses.CreateQuestionnaireResponse, error)
	UpdateQuestionnaireResponse(ctx context.Context, request *fhir_dto.QuestionnaireResponse) (*fhir_dto.QuestionnaireResponse, error)
	FindQuestionnaireResponseByID(ctx context.Context, questionnaireResponseID string) (*fhir_dto.QuestionnaireResponse, error)
	DeleteQuestionnaireResponseByID(ctx context.Context, questionnaireResponseID string) error
}

type QuestionnaireResponseRepository interface{}
