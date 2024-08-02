package organizations

import (
	"context"
	"konsulin-service/internal/pkg/dto/responses"
)

type OrganizationUsecase interface{}

type OrganizationRepository interface{}

type OrganizationFhirClient interface {
	ListOrganizations(ctx context.Context, page, row int) ([]responses.Organization, int, error)
}
