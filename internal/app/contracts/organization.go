package contracts

import (
	"context"
	"konsulin-service/internal/pkg/fhir_dto"
)

type OrganizationFhirClient interface {
	FindAll(ctx context.Context, organizationName, fetchType string, page, pageSize int) ([]fhir_dto.Organization, int, error)
	FindOrganizationByID(ctx context.Context, organizationID string) (*fhir_dto.Organization, error)
}
