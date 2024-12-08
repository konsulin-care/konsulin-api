package contracts

import (
	"context"
	"konsulin-service/internal/pkg/dto/responses"
	"konsulin-service/internal/pkg/fhir_dto"
)

type AssessmentUsecase interface {
	FindAll(ctx context.Context) ([]responses.Assessment, error)
	CreateAssessment(ctx context.Context, request *fhir_dto.Questionnaire) (*fhir_dto.Questionnaire, error)
	UpdateAssessment(ctx context.Context, request *fhir_dto.Questionnaire) (*fhir_dto.Questionnaire, error)
	FindAssessmentByID(ctx context.Context, questionnaireID string) (*fhir_dto.Questionnaire, error)
	DeleteAssessmentByID(ctx context.Context, questionnaireID string) error
}

type AssessmentRepository interface{}
