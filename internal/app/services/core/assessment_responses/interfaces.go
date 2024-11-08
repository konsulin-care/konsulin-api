package assessmentResponses

import (
	"context"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/dto/responses"
	"konsulin-service/internal/pkg/fhir_dto"
)

type AssessmentResponseUsecase interface {
	CreateAssessmentResponse(ctx context.Context, request *requests.CreateAssesmentResponse) (*responses.CreateAssessmentResponse, error)
	UpdateAssessmentResponse(ctx context.Context, request *fhir_dto.QuestionnaireResponse) (*fhir_dto.QuestionnaireResponse, error)
	FindAssessmentResponseByID(ctx context.Context, questionnaireResponseID string) (*responses.AssessmentResponse, error)
	DeleteAssessmentResponseByID(ctx context.Context, questionnaireResponseID string) error
}

type QuestionnaireResponseRepository interface{}
