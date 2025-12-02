package contracts

import (
	"context"
	"konsulin-service/internal/app/models"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/dto/responses"

	"github.com/go-playground/validator/v10"
)

type InitializeNewUserFHIRResourcesInput struct {
	Email            string `validate:"required,email"`
	SuperTokenUserID string

	// toogle to determine whether the underlying related FHIR resource should be created or not.
	PractitionerRolesExists bool
	PatientRolesExists      bool
	ClinicAdminRolesExists  bool
	ResearcherRolesExists   bool
	SuperadminRolesExists   bool
}

func (i *InitializeNewUserFHIRResourcesInput) Validate() error {
	return validator.New().Struct(i)
}

// ToogleByRoles toogle the correct toogle values based on the roles.
func (i *InitializeNewUserFHIRResourcesInput) ToogleByRoles(roles []string) {
	for _, role := range roles {
		switch role {
		case constvars.KonsulinRolePatient:
			i.PatientRolesExists = true
		case constvars.KonsulinRolePractitioner:
			i.PractitionerRolesExists = true
		case constvars.KonsulinRoleClinicAdmin:
			i.ClinicAdminRolesExists = true
		case constvars.KonsulinRoleResearcher:
			i.ResearcherRolesExists = true
		case constvars.KonsulinRoleSuperadmin:
			i.SuperadminRolesExists = true
		default:
			continue
		}
	}
}

// Resource translate to what resource should be created
// based on the toogle values.
func (i *InitializeNewUserFHIRResourcesInput) Resources() []string {
	resources := []string{}
	if i.PractitionerRolesExists {
		resources = append(resources, constvars.ResourcePractitioner)
	}
	if i.PatientRolesExists {
		resources = append(resources, constvars.ResourcePatient)
	}

	if i.ClinicAdminRolesExists || i.ResearcherRolesExists || i.SuperadminRolesExists {
		resources = append(resources, constvars.ResourcePerson)
	}

	return resources
}

type InitializeNewUserFHIRResourcesOutput struct {
	PatientID      string
	PractitionerID string
	PersonID       string
}

type UserUsecase interface {
	GetUserProfileBySession(ctx context.Context, sessionData string) (*responses.UserProfile, error)
	UpdateUserProfileBySession(ctx context.Context, sessionData string, request *requests.UpdateProfile) (*responses.UpdateUserProfile, error)
	DeleteUserBySession(ctx context.Context, sessionData string) error
	DeactivateUserBySession(ctx context.Context, sessionData string) error
	InitializeNewUserFHIRResources(ctx context.Context, input *InitializeNewUserFHIRResourcesInput) (*InitializeNewUserFHIRResourcesOutput, error)
}

type UserRepository interface {
	GetClient(ctx context.Context) (databaseClient interface{})
	CreateUser(ctx context.Context, userModel *models.User) (userID string, err error)
	FindByEmail(ctx context.Context, email string) (*models.User, error)
	FindByUsername(ctx context.Context, username string) (*models.User, error)
	FindByEmailOrUsername(ctx context.Context, email, username string) (*models.User, error)
	FindByWhatsAppNumber(ctx context.Context, whatsAppNumber string) (*models.User, error)
	FindByResetToken(ctx context.Context, token string) (*models.User, error)
	FindByID(ctx context.Context, userID string) (*models.User, error)
	UpdateUser(ctx context.Context, userModel *models.User) error
	DeleteByID(ctx context.Context, email string) error
}
