package contracts

import (
	"context"
	"konsulin-service/internal/pkg/fhir_dto"
)

type QuestionnaireResponseFhirClient interface {
	FindQuestionnaireResponseByID(ctx context.Context, questionnaireResponseID string) (*fhir_dto.QuestionnaireResponse, error)
	FindQuestionnaireResponsesByIdentifier(ctx context.Context, system, value string) ([]fhir_dto.QuestionnaireResponse, error)
}
