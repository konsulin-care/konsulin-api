package organizations

import (
	"context"
	"konsulin-service/internal/pkg/dto/responses"
)

type OrganizationUsecase interface{}

type OrganizationRepository interface{}

type OrganizationFhirClient interface {
	FindAll(ctx context.Context, page, pageSize int) ([]responses.Organization, int, error)
	FindOrganizationByID(ctx context.Context, organizationID string) (*responses.Organization, error)
}
