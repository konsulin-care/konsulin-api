package contracts

import (
	"context"
	"konsulin-service/internal/pkg/fhir_dto"
)

type PersonFhirClient interface {
	FindPersonByEmail(ctx context.Context, email string) ([]fhir_dto.Person, error)
}
