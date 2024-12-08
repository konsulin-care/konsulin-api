package contracts

import (
	"context"
	"konsulin-service/internal/pkg/fhir_dto"
)

type PractitionerFhirClient interface {
	CreatePractitioner(ctx context.Context, request *fhir_dto.Practitioner) (*fhir_dto.Practitioner, error)
	UpdatePractitioner(ctx context.Context, request *fhir_dto.Practitioner) (*fhir_dto.Practitioner, error)
	PatchPractitioner(ctx context.Context, request *fhir_dto.Practitioner) (*fhir_dto.Practitioner, error)
	FindPractitionerByID(ctx context.Context, PractitionerID string) (*fhir_dto.Practitioner, error)
}
