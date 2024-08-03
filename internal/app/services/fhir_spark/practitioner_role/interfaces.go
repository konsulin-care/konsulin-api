package practitionerRoles

import (
	"context"
	"konsulin-service/internal/pkg/dto/responses"
)

type PractitionerRoleUsecase interface{}

type PractitionerRoleRepository interface{}

type PractitionerRoleFhirClient interface {
	GetPractitionerRoleByOrganizationID(ctx context.Context, organizationID string) (*responses.PractitionerRole, error)
}
