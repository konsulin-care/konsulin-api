package clinics

import (
	"context"
	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/services/fhir_spark/organizations"
	practitionerRoles "konsulin-service/internal/app/services/fhir_spark/practitioner_role"
	"konsulin-service/internal/app/services/fhir_spark/practitioners"
	"konsulin-service/internal/app/services/fhir_spark/schedules"
	"konsulin-service/internal/app/services/shared/redis"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/dto/responses"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/fhir_dto"
	"konsulin-service/internal/pkg/utils"
)

type clinicUsecase struct {
	OrganizationFhirClient     organizations.OrganizationFhirClient
	PractitionerRoleFhirClient practitionerRoles.PractitionerRoleFhirClient
	PractitionerFhirClient     practitioners.PractitionerFhirClient
	ScheduleFhirClient         schedules.ScheduleFhirClient
	RedisRepository            redis.RedisRepository
	InternalConfig             *config.InternalConfig
}

func NewClinicUsecase(
	organizationFhirClient organizations.OrganizationFhirClient,
	practitionerRoleFhirClient practitionerRoles.PractitionerRoleFhirClient,
	practitionerFhirClient practitioners.PractitionerFhirClient,
	scheduleFhirClient schedules.ScheduleFhirClient,
	redisRepository redis.RedisRepository,
	internalConfig *config.InternalConfig,
) ClinicUsecase {
	return &clinicUsecase{
		OrganizationFhirClient:     organizationFhirClient,
		PractitionerFhirClient:     practitionerFhirClient,
		PractitionerRoleFhirClient: practitionerRoleFhirClient,
		ScheduleFhirClient:         scheduleFhirClient,
		RedisRepository:            redisRepository,
		InternalConfig:             internalConfig,
	}
}

func (uc *clinicUsecase) FindAll(ctx context.Context, nameFilter, fetchType string, page, pageSize int) ([]responses.Clinic, *responses.Pagination, error) {
	organizationsFhir, totalData, err := uc.OrganizationFhirClient.FindAll(ctx, nameFilter, fetchType, page, pageSize)
	if err != nil {
		return nil, nil, err
	}
	// Build the response
	response := make([]responses.Clinic, len(organizationsFhir))
	for i, eachOrganization := range organizationsFhir {
		response[i] = utils.ConvertOrganizationToClinicResponse(eachOrganization)
	}

	if fetchType == constvars.FhirFetchResourceTypePaged {
		paginationData := utils.BuildPaginationResponse(totalData, page, pageSize, uc.InternalConfig.App.BaseUrl+constvars.ResourceClinics)
		return response, paginationData, nil
	}

	return response, nil, nil
}

func (uc *clinicUsecase) FindAllCliniciansByClinicID(ctx context.Context, request *requests.FindAllCliniciansByClinicID) ([]responses.ClinicClinician, *responses.Pagination, error) {
	practitionerRoles, err := uc.PractitionerRoleFhirClient.FindPractitionerRoleByCustomRequest(ctx, request)
	if err != nil {
		return nil, nil, err
	}

	clinicians, paginationData, err := uc.fetchAllCliniciansByPractitionerRoles(ctx, practitionerRoles)
	if err != nil {
		return nil, nil, err
	}

	return clinicians, paginationData, nil
}

func (uc *clinicUsecase) fetchAllCliniciansByPractitionerRoles(ctx context.Context, practitionerRoles []fhir_dto.PractitionerRole) ([]responses.ClinicClinician, *responses.Pagination, error) {
	var clinicians []responses.ClinicClinician
	for _, practitionerRole := range practitionerRoles {
		if practitionerRole.Practitioner.Reference != "" && practitionerRole.Active {
			practitionerID := practitionerRole.Practitioner.Reference[len("Practitioner/"):]
			practitioner, err := uc.PractitionerFhirClient.FindPractitionerByID(ctx, practitionerID)
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
}

func (uc *clinicUsecase) FindByID(ctx context.Context, clinicID string) (*responses.Clinic, error) {
	organization, err := uc.OrganizationFhirClient.FindOrganizationByID(ctx, clinicID)
	if err != nil {
		return nil, err
	}
	// Build the response
	response := utils.ConvertOrganizationToClinicDetailResponse(*organization)

	return &response, nil
}

func (uc *clinicUsecase) FindClinicianByClinicAndClinicianID(ctx context.Context, clinicID, clinicianID string) (*responses.ClinicianSummary, error) {
	practitioner, err := uc.PractitionerFhirClient.FindPractitionerByID(ctx, clinicianID)
	if err != nil {
		return nil, err
	}

	practitionerRoles, err := uc.PractitionerRoleFhirClient.FindPractitionerRoleByPractitionerIDAndOrganizationID(ctx, practitioner.ID, clinicID)
	if err != nil {
		return nil, err
	}

	organization, err := uc.OrganizationFhirClient.FindOrganizationByID(ctx, clinicID)
	if err != nil {
		return nil, err
	}

	schedules, err := uc.ScheduleFhirClient.FindScheduleByPractitionerRoleID(ctx, practitionerRoles[0].ID)
	if err != nil {
		return nil, err
	}

	if len(schedules) > 1 {
		return nil, exceptions.ErrGetFHIRResourceDuplicate(nil, constvars.ResourceSchedule)
	}

	practiceInformation := responses.PracticeInformation{
		Affiliation: organization.Name,
		Specialties: utils.ExtractSpecialtiesText(practitionerRoles[0].Specialty),
		PricePerSession: responses.PricePerSession{
			Value:    practitionerRoles[0].Extension[0].ValueMoney.Value,
			Currency: practitionerRoles[0].Extension[0].ValueMoney.Currency,
		},
	}

	response := &responses.ClinicianSummary{
		ClinicianID:         practitioner.ID,
		PractitionerRoleID:  practitionerRoles[0].ID,
		Name:                utils.GetFullName(practitioner.Name),
		PracticeInformation: practiceInformation,
	}

	if len(schedules) == 1 {
		response.ScheduleID = schedules[0].ID
	}

	return response, nil
}
