package contracts

import (
	"context"
	"konsulin-service/internal/pkg/fhir_dto"
)

type ServiceRequestFhirClient interface {
	CreateServiceRequest(ctx context.Context, request *fhir_dto.CreateServiceRequestInput) (*fhir_dto.CreateServiceRequestOutput, error)
	GetServiceRequestByIDAndVersion(ctx context.Context, id string, version string) (*fhir_dto.GetServiceRequestOutput, error)
	Search(ctx context.Context, input *fhir_dto.SearchServiceRequestInput) ([]fhir_dto.GetServiceRequestOutput, error)
	Update(ctx context.Context, id string, input *fhir_dto.UpdateServiceRequestInput) (*fhir_dto.GetServiceRequestOutput, error)
	EnsureAllNecessaryGroupsExists(ctx context.Context) error
}
