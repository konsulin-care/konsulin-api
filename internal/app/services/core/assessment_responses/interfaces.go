package assessmentResponses

import (
	"context"
	fhir_dto "konsulin-service/internal/pkg/dto/fhir"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/dto/responses"
)

type AssessmentResponseUsecase interface {
	CreateAssessmentResponse(ctx context.Context, request *requests.CreateAssesmentResponse) (*responses.CreateAssessmentResponse, error)
	UpdateAssessmentResponse(ctx context.Context, request *fhir_dto.QuestionnaireResponse) (*fhir_dto.QuestionnaireResponse, error)
	FindAssessmentResponseByID(ctx context.Context, questionnaireResponseID string) (*fhir_dto.QuestionnaireResponse, error)
	DeleteAssessmentResponseByID(ctx context.Context, questionnaireResponseID string) error
}

type QuestionnaireResponseRepository interface{}
