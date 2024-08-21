package practitionerRoles

import (
	"context"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/dto/responses"
)

type PractitionerRoleUsecase interface{}

type PractitionerRoleRepository interface{}

type PractitionerRoleFhirClient interface {
	DeletePractitionerRoleByID(ctx context.Context, practitionerRoleID string) error
	FindPractitionerRoleByOrganizationID(ctx context.Context, organizationID string) ([]responses.PractitionerRole, error)
	FindPractitionerRoleByPractitionerID(ctx context.Context, practitionerID string) ([]responses.PractitionerRole, error)
	FindPractitionerRoleByPractitionerIDAndOrganizationID(ctx context.Context, practitionerID, organizationID string) ([]responses.PractitionerRole, error)
	CreatePractitionerRoles(ctx context.Context, request interface{}) error
	CreatePractitionerRole(ctx context.Context, request *requests.PractitionerRole) (*responses.PractitionerRole, error)
	UpdatePractitionerRole(ctx context.Context, request *requests.PractitionerRole) (*responses.PractitionerRole, error)
	FindPractitionerRoleByPractitionerIDAndName(ctx context.Context, request *requests.GetClinicianByClinicianID) ([]responses.PractitionerRole, error)
	FindPractitionerRoleByID(ctx context.Context, practitionerRoleID string) (*responses.PractitionerRole, error)
}
