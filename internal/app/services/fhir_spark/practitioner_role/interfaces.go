package practitionerRoles

import (
	"context"
	"konsulin-service/internal/pkg/dto/responses"
)

type PractitionerRoleUsecase interface{}

type PractitionerRoleRepository interface{}

type PractitionerRoleFhirClient interface {
	DeletePractitionerRoleByID(ctx context.Context, practitionerRoleID string) error
	FindPractitionerRoleByOrganizationID(ctx context.Context, organizationID string) ([]responses.PractitionerRole, error)
	FindPractitionerRoleByPractitionerID(ctx context.Context, practitionerID string) ([]responses.PractitionerRole, error)
	FindPractitionerRoleByPractitionerIDAndOrganizationID(ctx context.Context, practitionerID, organizationID string) (*responses.PractitionerRole, error)
	CreatePractitionerRoles(ctx context.Context, request interface{}) error
}
