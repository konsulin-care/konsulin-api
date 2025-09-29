package contracts

import (
	"context"
	"konsulin-service/internal/pkg/fhir_dto"
)

type ServiceRequestFhirClient interface {
	CreateServiceRequest(ctx context.Context, request *fhir_dto.CreateServiceRequestInput) (*fhir_dto.CreateServiceRequestOutput, error)
	GetServiceRequestByIDAndVersion(ctx context.Context, id string, version string) (*fhir_dto.GetServiceRequestOutput, error)
	EnsureAllNecessaryGroupsExists(ctx context.Context) error
}
