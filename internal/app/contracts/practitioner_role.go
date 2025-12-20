package contracts

import (
	"context"
	"fmt"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/fhir_dto"
	"net/url"
	"strings"
)

type PractitionerRoleFhirClient interface {
	DeletePractitionerRoleByID(ctx context.Context, practitionerRoleID string) error
	FindPractitionerRoleByOrganizationID(ctx context.Context, organizationID string) ([]fhir_dto.PractitionerRole, error)
	FindPractitionerRoleByCustomRequest(ctx context.Context, request *requests.FindAllCliniciansByClinicID) ([]fhir_dto.PractitionerRole, error)
	FindPractitionerRoleByPractitionerID(ctx context.Context, practitionerID string) ([]fhir_dto.PractitionerRole, error)
	FindPractitionerRoleByPractitionerIDAndOrganizationID(ctx context.Context, practitionerID, organizationID string) ([]fhir_dto.PractitionerRole, error)
	CreatePractitionerRoles(ctx context.Context, request interface{}) error
	CreatePractitionerRole(ctx context.Context, request *fhir_dto.PractitionerRole) (*fhir_dto.PractitionerRole, error)
	UpdatePractitionerRole(ctx context.Context, request *fhir_dto.PractitionerRole) (*fhir_dto.PractitionerRole, error)
	FindPractitionerRoleByPractitionerIDAndName(ctx context.Context, request *requests.FindClinicianByClinicianID) ([]fhir_dto.PractitionerRole, error)
	FindPractitionerRoleByID(ctx context.Context, practitionerRoleID string) (*fhir_dto.PractitionerRole, error)
	Search(ctx context.Context, params PractitionerRoleSearchParams) ([]fhir_dto.PractitionerRole, error)
}

type PractitionerRoleSearchParams struct {
	Active         *bool
	PractitionerID string
	OrganizationID string
	Elements       []string
}

// ToQueryParam converts PractitionerRoleSearchParams into URL query parameters.
func (p PractitionerRoleSearchParams) ToQueryParam() url.Values {
	q := url.Values{}
	if p.Active != nil {
		if *p.Active {
			q.Add("active", "true")
		} else {
			q.Add("active", "false")
		}
	}
	if p.PractitionerID != "" {
		q.Add("practitioner", fmt.Sprintf("Practitioner/%s", p.PractitionerID))
	}
	if p.OrganizationID != "" {
		q.Add("organization", fmt.Sprintf("Organization/%s", p.OrganizationID))
	}
	if len(p.Elements) > 0 {
		q.Add("_elements", strings.Join(p.Elements, ","))
	}
	return q
}
