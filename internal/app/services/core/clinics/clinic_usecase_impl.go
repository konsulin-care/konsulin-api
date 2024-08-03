package clinics

import (
	"context"
	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/services/fhir_spark/organizations"
	"konsulin-service/internal/app/services/shared/redis"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/responses"
	"konsulin-service/internal/pkg/utils"
)

type clinicUsecase struct {
	OrganizationFhirClient organizations.OrganizationFhirClient
	RedisRepository        redis.RedisRepository
	InternalConfig         *config.InternalConfig
}

func NewClinicUsecase(
	organizationFhirClient organizations.OrganizationFhirClient,
	redisRepository redis.RedisRepository,
	internalConfig *config.InternalConfig,
) ClinicUsecase {
	return &clinicUsecase{
		OrganizationFhirClient: organizationFhirClient,
		RedisRepository:        redisRepository,
		InternalConfig:         internalConfig,
	}
}

func (uc *clinicUsecase) FindAll(ctx context.Context, page, pageSize int) ([]responses.Clinic, *responses.Pagination, error) {
	organizationsFhir, totalData, err := uc.OrganizationFhirClient.FindAll(ctx, page, pageSize)
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

func (uc *clinicUsecase) FindByID(ctx context.Context, clinicID string) (*responses.Clinic, error) {
	organization, err := uc.OrganizationFhirClient.FindOrganizationByID(ctx, clinicID)
	if err != nil {
		return nil, err
	}
	// Build the response
	response := organization.ConvertToClinicResponse()

	return &response, nil
}
