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
	FindPractitionerByIdentifier(ctx context.Context, system, value string) ([]fhir_dto.Practitioner, error)
	FindPractitionerByEmail(ctx context.Context, email string) ([]fhir_dto.Practitioner, error)
	FindPractitionerByPhone(ctx context.Context, phone string) ([]fhir_dto.Practitioner, error)
}
