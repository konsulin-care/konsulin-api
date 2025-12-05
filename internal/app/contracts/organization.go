package contracts

import (
	"context"
	"konsulin-service/internal/pkg/fhir_dto"
)

type OrganizationFhirClient interface {
	FindAll(ctx context.Context, organizationName, fetchType string, page, pageSize int) ([]fhir_dto.Organization, int, error)
	FindOrganizationByID(ctx context.Context, organizationID string) (*fhir_dto.Organization, error)
	Update(ctx context.Context, organization fhir_dto.Organization) (*fhir_dto.Organization, error)
}

// RegisterPractitionerRoleInput captures the minimal data required to
// register a PractitionerRole and Schedule for a practitioner within
// a given organization.
type RegisterPractitionerRoleInput struct {
	OrganizationID string
	Email          string
}

// RegisterPractitionerRoleOutput returns identifiers of the related
// FHIR resources that were created as part of the registration flow.
type RegisterPractitionerRoleOutput struct {
	PractitionerID     string
	PractitionerRoleID string
	ScheduleID         string
}

// OrganizationUsecase defines high-level organization-related behaviors.
type OrganizationUsecase interface {
	// RegisterPractitionerRoleAndSchedule links an existing or newly created
	// Practitioner (resolved by email) to an Organization by creating a
	// PractitionerRole and Schedule in FHIR, subject to role and org checks.
	RegisterPractitionerRoleAndSchedule(ctx context.Context, in RegisterPractitionerRoleInput) (*RegisterPractitionerRoleOutput, error)
}
