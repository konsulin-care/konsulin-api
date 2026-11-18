package contracts

import (
	"context"

	"github.com/konsulin-care/konsulin-api/internal/pkg/fhir_dto"
)

type QuestionnaireResponseFhirClient interface {
	FindQuestionnaireResponseByID(ctx context.Context, questionnaireResponseID string) (*fhir_dto.QuestionnaireResponse, error)
	FindQuestionnaireResponsesByIdentifier(ctx context.Context, system, value string) ([]fhir_dto.QuestionnaireResponse, error)
}
