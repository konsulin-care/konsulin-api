package contracts

import (
	"context"
	"konsulin-service/internal/pkg/fhir_dto"
)

type ServiceRequestFhirClient interface {
	CreateServiceRequest(ctx context.Context, request *fhir_dto.CreateServiceRequestInput) (*fhir_dto.CreateServiceRequestOutput, error)
}
