package clinics

import (
	"context"
	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/services/fhir_spark/organizations"
	practitionerRoles "konsulin-service/internal/app/services/fhir_spark/practitioner_role"
	"konsulin-service/internal/app/services/fhir_spark/practitioners"
	"konsulin-service/internal/app/services/shared/redis"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/responses"
	"konsulin-service/internal/pkg/utils"
)

type clinicUsecase struct {
	OrganizationFhirClient organizations.OrganizationFhirClient
	PractitionerRoleClient practitionerRoles.PractitionerRoleFhirClient
	PractitionerClient     practitioners.PractitionerFhirClient
	RedisRepository        redis.RedisRepository
	InternalConfig         *config.InternalConfig
}

func NewClinicUsecase(
	organizationFhirClient organizations.OrganizationFhirClient,
	practitionerRoleFhirClient practitionerRoles.PractitionerRoleFhirClient,
	practitionerFhirClient practitioners.PractitionerFhirClient,
	redisRepository redis.RedisRepository,
	internalConfig *config.InternalConfig,
) ClinicUsecase {
	return &clinicUsecase{
		OrganizationFhirClient: organizationFhirClient,
		PractitionerClient:     practitionerFhirClient,
		PractitionerRoleClient: practitionerRoleFhirClient,
		RedisRepository:        redisRepository,
		InternalConfig:         internalConfig,
	}
}

func (uc *clinicUsecase) FindAll(ctx context.Context, nameFilter string, page, pageSize int) ([]responses.Clinic, *responses.Pagination, error) {
	organizationsFhir, totalData, err := uc.OrganizationFhirClient.FindAll(ctx, nameFilter, page, pageSize)
	if err != nil {
		return nil, nil, err
	}
	// Build the response
	response := make([]responses.Clinic, len(organizationsFhir))
	for i, eachOrganization := range organizationsFhir {
		response[i] = eachOrganization.ConvertToClinicResponse()
	}

	paginationData := utils.BuildPaginationResponse(totalData, page, pageSize, uc.InternalConfig.App.BaseUrl+constvars.ResourceClinics)

	return response, paginationData, nil
}

func (uc *clinicUsecase) FindAllCliniciansByClinicID(ctx context.Context, nameFilter, clinicID string, page, pageSize int) ([]responses.ClinicClinician, *responses.Pagination, error) {
	practitionerRoles, err := uc.PractitionerRoleClient.FindPractitionerRoleByOrganizationID(ctx, clinicID)
	if err != nil {
		return nil, nil, err
	}

	clinicians, paginationData, err := uc.fetchAllCliniciansByPractitionerRoles(ctx, practitionerRoles, nameFilter, page, pageSize)
	if err != nil {
		return nil, nil, err
	}

	return clinicians, paginationData, nil
}

func (uc *clinicUsecase) fetchAllCliniciansByPractitionerRoles(ctx context.Context, practitionerRoles []responses.PractitionerRole, nameFilter string, page, pageSize int) ([]responses.ClinicClinician, *responses.Pagination, error) {
	var clinicians []responses.ClinicClinician
	for _, practitionerRole := range practitionerRoles {
		if practitionerRole.Practitioner.Reference != "" {
			practitionerID := practitionerRole.Practitioner.Reference[len("Practitioner/"):]
			practitioner, err := uc.PractitionerClient.FindPractitionerByID(ctx, practitionerID)
			if err != nil {
				return nil, nil, err
			}
			clinicians = append(clinicians, utils.MapPractitionerToClinicClinician(
				practitioner,
				practitionerRole.Specialty,
				practitionerRole.Organization.Display,
			))
		}
	}
	return clinicians, nil, nil

	// // Build the response
	// response := make([]responses.Clinic, len(organizationsFhir))
	// for i, eachOrganization := range organizationsFhir {
	// 	response[i] = eachOrganization.ConvertToClinicResponse()
	// }

	// paginationData := utils.BuildPaginationResponse(totalData, page, pageSize, uc.InternalConfig.App.BaseUrl+constvars.ResourceClinics)

	// return response, paginationData, nil
}

func (uc *clinicUsecase) FindByID(ctx context.Context, clinicID string) (*responses.Clinic, error) {
	organization, err := uc.OrganizationFhirClient.FindOrganizationByID(ctx, clinicID)
	if err != nil {
		return nil, err
	}
	// Build the response
	response := organization.ConvertToClinicDetailResponse()

	return &response, nil
}
