package clinics

import (
	"context"
	"fmt"
	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/dto/responses"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/fhir_dto"
	"konsulin-service/internal/pkg/utils"
	"sync"

	"go.uber.org/zap"
)

type clinicUsecase struct {
	OrganizationFhirClient     contracts.OrganizationFhirClient
	PractitionerRoleFhirClient contracts.PractitionerRoleFhirClient
	PractitionerFhirClient     contracts.PractitionerFhirClient
	ScheduleFhirClient         contracts.ScheduleFhirClient
	ChargeItemDefinition       contracts.ChargeItemDefinitionFhirClient
	RedisRepository            contracts.RedisRepository
	InternalConfig             *config.InternalConfig
	Log                        *zap.Logger
}

var (
	clinicUsecaseInstance contracts.ClinicUsecase
	onceClinicUsecase     sync.Once
)

func NewClinicUsecase(
	organizationFhirClient contracts.OrganizationFhirClient,
	practitionerRoleFhirClient contracts.PractitionerRoleFhirClient,
	practitionerFhirClient contracts.PractitionerFhirClient,
	scheduleFhirClient contracts.ScheduleFhirClient,
	chargeItemDefinition contracts.ChargeItemDefinitionFhirClient,
	redisRepository contracts.RedisRepository,
	internalConfig *config.InternalConfig,
	logger *zap.Logger,
) contracts.ClinicUsecase {
	onceClinicUsecase.Do(func() {
		instance := &clinicUsecase{
			OrganizationFhirClient:     organizationFhirClient,
			PractitionerFhirClient:     practitionerFhirClient,
			PractitionerRoleFhirClient: practitionerRoleFhirClient,
			ScheduleFhirClient:         scheduleFhirClient,
			ChargeItemDefinition:       chargeItemDefinition,
			RedisRepository:            redisRepository,
			InternalConfig:             internalConfig,
			Log:                        logger,
		}
		clinicUsecaseInstance = instance
	})
	return clinicUsecaseInstance
}

func (uc *clinicUsecase) FindAll(ctx context.Context, nameFilter, fetchType string, page, pageSize int) ([]responses.Clinic, *responses.Pagination, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	uc.Log.Info("clinicUsecase.FindAll called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	organizationsFhir, totalData, err := uc.OrganizationFhirClient.FindAll(ctx, nameFilter, fetchType, page, pageSize)
	if err != nil {
		uc.Log.Error("clinicUsecase.FindAll error fetching organizations from FHIR client",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, nil, err
	}

	response := make([]responses.Clinic, len(organizationsFhir))
	for i, eachOrganization := range organizationsFhir {
		response[i] = utils.ConvertOrganizationToClinicResponse(eachOrganization)
	}

	if fetchType == constvars.FhirFetchResourceTypePaged {
		paginationData := utils.BuildPaginationResponse(totalData, page, pageSize, uc.InternalConfig.App.BaseUrl+constvars.ResourceClinics)
		uc.Log.Info("clinicUsecase.FindAll succeeded with pagination",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Int(constvars.LoggingResponseCountKey, totalData),
		)
		return response, paginationData, nil
	}

	uc.Log.Info("clinicUsecase.FindAll succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Int(constvars.LoggingResponseCountKey, totalData),
	)
	return response, nil, nil
}

func (uc *clinicUsecase) FindAllCliniciansByClinicID(ctx context.Context, request *requests.FindAllCliniciansByClinicID) ([]responses.ClinicClinician, *responses.Pagination, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	uc.Log.Info("clinicUsecase.FindAllCliniciansByClinicID called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingClinicIDKey, request.ClinicID),
	)

	practitionerRoles, err := uc.PractitionerRoleFhirClient.FindPractitionerRoleByCustomRequest(ctx, request)
	if err != nil {
		uc.Log.Error("clinicUsecase.FindAllCliniciansByClinicID error fetching practitioner roles",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, nil, err
	}

	clinicians, paginationData, err := uc.fetchAllCliniciansByPractitionerRoles(ctx, practitionerRoles)
	if err != nil {
		uc.Log.Error("clinicUsecase.FindAllCliniciansByClinicID error fetching clinicians",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, nil, err
	}

	uc.Log.Info("clinicUsecase.FindAllCliniciansByClinicID succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Int(constvars.LoggingClinicianCountKey, len(clinicians)),
	)
	return clinicians, paginationData, nil
}

func (uc *clinicUsecase) fetchAllCliniciansByPractitionerRoles(ctx context.Context, practitionerRoles []fhir_dto.PractitionerRole) ([]responses.ClinicClinician, *responses.Pagination, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	uc.Log.Info("clinicUsecase.fetchAllCliniciansByPractitionerRoles called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Int(constvars.LoggingPractitionerRoleCountKey, len(practitionerRoles)),
	)
	var clinicians []responses.ClinicClinician
	for _, practitionerRole := range practitionerRoles {
		if practitionerRole.Practitioner.Reference != "" && practitionerRole.Active {
			practitionerID := practitionerRole.Practitioner.Reference[len("Practitioner/"):]
			practitioner, err := uc.PractitionerFhirClient.FindPractitionerByID(ctx, practitionerID)
			if err != nil {
				uc.Log.Error("clinicUsecase.fetchAllCliniciansByPractitionerRoles error fetching practitioner",
					zap.String(constvars.LoggingRequestIDKey, requestID),
					zap.String(constvars.LoggingPractitionerIDKey, practitionerID),
					zap.Error(err),
				)
				return nil, nil, err
			}
			clinician := utils.MapPractitionerToClinicClinician(
				practitioner,
				practitionerRole.Specialty,
				practitionerRole.Organization.Display,
			)
			clinicians = append(clinicians, clinician)
			uc.Log.Info("clinicUsecase.fetchAllCliniciansByPractitionerRoles processed clinician",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.String(constvars.LoggingPractitionerIDKey, practitionerID),
			)
		}
	}
	uc.Log.Info("clinicUsecase.fetchAllCliniciansByPractitionerRoles succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Int("clinicians_found", len(clinicians)),
	)
	return clinicians, nil, nil
}

func (uc *clinicUsecase) FindByID(ctx context.Context, clinicID string) (*responses.Clinic, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	uc.Log.Info("clinicUsecase.FindByID called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingClinicIDKey, clinicID),
	)

	organization, err := uc.OrganizationFhirClient.FindOrganizationByID(ctx, clinicID)
	if err != nil {
		uc.Log.Error("clinicUsecase.FindByID error fetching organization",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingClinicIDKey, clinicID),
			zap.Error(err),
		)
		return nil, err
	}

	response := utils.ConvertOrganizationToClinicDetailResponse(*organization)
	uc.Log.Info("clinicUsecase.FindByID succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingClinicIDKey, clinicID),
	)
	return &response, nil
}

func (uc *clinicUsecase) FindClinicianByClinicAndClinicianID(ctx context.Context, clinicID, clinicianID string) (*responses.ClinicianSummary, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	uc.Log.Info("clinicUsecase.FindClinicianByClinicAndClinicianID called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingClinicIDKey, clinicID),
		zap.String(constvars.LoggingClinicianIDKey, clinicianID),
	)

	practitioner, err := uc.PractitionerFhirClient.FindPractitionerByID(ctx, clinicianID)
	if err != nil {
		uc.Log.Error("clinicUsecase.FindClinicianByClinicAndClinicianID error fetching practitioner",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingClinicianIDKey, clinicianID),
			zap.Error(err),
		)
		return nil, err
	}

	practitionerRoles, err := uc.PractitionerRoleFhirClient.FindPractitionerRoleByPractitionerIDAndOrganizationID(ctx, practitioner.ID, clinicID)
	if err != nil {
		uc.Log.Error("clinicUsecase.FindClinicianByClinicAndClinicianID error fetching practitioner roles",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingClinicIDKey, clinicID),
			zap.String(constvars.LoggingPractitionerIDKey, practitioner.ID),
			zap.Error(err),
		)
		return nil, err
	}

	if len(practitionerRoles) > 1 {
		uc.Log.Error("clinicUsecase.FindClinicianByClinicAndClinicianID duplicate practitioner roles found",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingClinicIDKey, clinicID),
			zap.String(constvars.LoggingPractitionerIDKey, practitioner.ID),
		)
		return nil, exceptions.ErrGetFHIRResourceDuplicate(nil, constvars.ResourcePractitionerRole)
	}

	organization, err := uc.OrganizationFhirClient.FindOrganizationByID(ctx, clinicID)
	if err != nil {
		uc.Log.Error("clinicUsecase.FindClinicianByClinicAndClinicianID error fetching organization",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingClinicIDKey, clinicID),
			zap.Error(err),
		)
		return nil, err
	}

	schedules, err := uc.ScheduleFhirClient.FindScheduleByPractitionerRoleID(ctx, practitionerRoles[0].ID)
	if err != nil {
		uc.Log.Error("clinicUsecase.FindClinicianByClinicAndClinicianID error fetching schedules",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingPractitionerRoleIDKey, practitionerRoles[0].ID),
			zap.Error(err),
		)
		return nil, err
	}

	practitionerRoleFullResourceID := utils.ParseSlashSeparatedToDashSeparated(fmt.Sprintf("%s/%s", constvars.ResourcePractitionerRole, practitionerRoles[0].ID))
	chargeItemDefinition, err := uc.ChargeItemDefinition.FindChargeItemDefinitionByPractitionerRoleID(ctx, practitionerRoleFullResourceID)
	if err != nil {
		uc.Log.Error("clinicUsecase.FindClinicianByClinicAndClinicianID error fetching charge item definition",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String("practitioner_role_resource_id", practitionerRoleFullResourceID),
			zap.Error(err),
		)
		return nil, err
	}

	if len(schedules) > 1 {
		uc.Log.Error("clinicUsecase.FindClinicianByClinicAndClinicianID duplicate schedules found",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingPractitionerRoleIDKey, practitionerRoles[0].ID),
		)
		return nil, exceptions.ErrGetFHIRResourceDuplicate(nil, constvars.ResourceSchedule)
	}

	practiceInformation := responses.PracticeInformation{
		Affiliation: organization.Name,
		Specialties: utils.ExtractSpecialtiesText(practitionerRoles[0].Specialty),
		PricePerSession: responses.PricePerSession{
			Value:    chargeItemDefinition.PropertyGroup[0].PriceComponent[0].Amount.Value,
			Currency: chargeItemDefinition.PropertyGroup[0].PriceComponent[0].Amount.Currency,
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

	uc.Log.Info("clinicUsecase.FindClinicianByClinicAndClinicianID succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingClinicIDKey, clinicID),
		zap.String(constvars.LoggingClinicianIDKey, clinicianID),
	)
	return response, nil
}
