package clinicians

import (
	"context"
	"errors"
	"fmt"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/app/models"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/dto/responses"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/fhir_dto"
	"konsulin-service/internal/pkg/utils"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

type clinicianUsecase struct {
	PractitionerFhirClient         contracts.PractitionerFhirClient
	PractitionerRoleFhirClient     contracts.PractitionerRoleFhirClient
	OrganizationFhirClient         contracts.OrganizationFhirClient
	ScheduleFhirClient             contracts.ScheduleFhirClient
	SlotFhirClient                 contracts.SlotFhirClient
	AppointmentFhirClient          contracts.AppointmentFhirClient
	ChargeItemDefinitionFhirClient contracts.ChargeItemDefinitionFhirClient
	SessionService                 contracts.SessionService
	Log                            *zap.Logger
}

var (
	clinicianUsecaseInstance contracts.ClinicianUsecase
	onceClinicianUsecase     sync.Once
)

func NewClinicianUsecase(
	practitionerFhirClient contracts.PractitionerFhirClient,
	practitionerRoleFhirClient contracts.PractitionerRoleFhirClient,
	organizationFhirClient contracts.OrganizationFhirClient,
	scheduleFhirClient contracts.ScheduleFhirClient,
	slotFhirClient contracts.SlotFhirClient,
	appointmentFhirClient contracts.AppointmentFhirClient,
	chargeItemDefinitionFhirClient contracts.ChargeItemDefinitionFhirClient,
	sessionService contracts.SessionService,
	logger *zap.Logger,
) contracts.ClinicianUsecase {
	onceClinicianUsecase.Do(func() {
		instance := &clinicianUsecase{
			PractitionerFhirClient:         practitionerFhirClient,
			PractitionerRoleFhirClient:     practitionerRoleFhirClient,
			OrganizationFhirClient:         organizationFhirClient,
			ScheduleFhirClient:             scheduleFhirClient,
			SlotFhirClient:                 slotFhirClient,
			AppointmentFhirClient:          appointmentFhirClient,
			ChargeItemDefinitionFhirClient: chargeItemDefinitionFhirClient,
			SessionService:                 sessionService,
			Log:                            logger,
		}
		clinicianUsecaseInstance = instance
	})
	return clinicianUsecaseInstance
}
func (uc *clinicianUsecase) DeleteClinicByID(ctx context.Context, sessionData, clinicID string) error {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	uc.Log.Info("clinicianUsecase.DeleteClinicByID called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingClinicIDKey, clinicID),
	)

	session, err := uc.SessionService.ParseSessionData(ctx, sessionData)
	if err != nil {
		uc.Log.Error("clinicianUsecase.DeleteClinicByID error parsing session data",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return err
	}

	if session.IsNotPractitioner() {
		uc.Log.Error("clinicianUsecase.DeleteClinicByID role mismatch: not a practitioner",
			zap.String(constvars.LoggingRequestIDKey, requestID),
		)
		return exceptions.ErrNotMatchRoleType(nil)
	}

	practitionerRoles, err := uc.PractitionerRoleFhirClient.FindPractitionerRoleByPractitionerIDAndOrganizationID(ctx, session.PractitionerID, clinicID)
	if err != nil {
		uc.Log.Error("clinicianUsecase.DeleteClinicByID error fetching practitioner roles",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingPractitionerIDKey, session.PractitionerID),
			zap.String(constvars.LoggingClinicIDKey, clinicID),
			zap.Error(err),
		)
		return err
	}

	if len(practitionerRoles) > 1 {
		fhirError := fmt.Errorf("duplicate result for practitionerID: %s clinicID: %s", session.PractitionerID, clinicID)
		uc.Log.Error("clinicianUsecase.DeleteClinicByID duplicate practitioner roles found",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingPractitionerIDKey, session.PractitionerID),
			zap.String(constvars.LoggingClinicIDKey, clinicID),
			zap.Error(fhirError),
		)
		return exceptions.ErrGetFHIRResourceDuplicate(fhirError, constvars.ResourcePractitionerRole)
	}

	schedules, err := uc.ScheduleFhirClient.FindScheduleByPractitionerRoleID(ctx, practitionerRoles[0].ID)
	if err != nil {
		uc.Log.Error("clinicianUsecase.DeleteClinicByID error fetching schedule",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingPractitionerRoleIDKey, practitionerRoles[0].ID),
			zap.Error(err),
		)
		return fmt.Errorf("error fetching Schedule: %w", err)
	}

	if len(schedules) > 1 {
		fhirError := fmt.Errorf("duplicate result for practitionerRoleID: %s", practitionerRoles[0].ID)
		uc.Log.Error("clinicianUsecase.DeleteClinicByID duplicate schedules found",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingPractitionerRoleIDKey, practitionerRoles[0].ID),
			zap.Error(fhirError),
		)
		return exceptions.ErrGetFHIRResourceDuplicate(fhirError, constvars.ResourceSchedule)
	}

	slots, err := uc.SlotFhirClient.FindSlotByScheduleIDAndStatus(ctx, schedules[0].ID, constvars.FhirSlotStatusBusy)
	if err != nil {
		uc.Log.Error("clinicianUsecase.DeleteClinicByID error fetching busy slots",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingScheduleIDKey, schedules[0].ID),
			zap.Error(err),
		)
		return err
	}

	if len(slots) > 0 {
		customErrMessage := errors.New("you can't delete this clinic from your practice, you still have on-goind appointments")
		uc.Log.Error("clinicianUsecase.DeleteClinicByID busy slots found; clinic cannot be deleted",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingScheduleIDKey, schedules[0].ID),
			zap.Error(customErrMessage),
		)
		return exceptions.ErrClientCustomMessage(customErrMessage)
	}

	uc.Log.Info("clinicianUsecase.DeleteClinicByID succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingClinicIDKey, clinicID),
	)
	return nil
}

func (uc *clinicianUsecase) CreatePracticeInformation(ctx context.Context, sessionData string, req *requests.CreatePracticeInformation) ([]responses.PracticeInformation, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	uc.Log.Info("clinicianUsecase.CreatePracticeInformation called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	session, err := uc.parseAndValidatePractitionerSession(ctx, sessionData)
	if err != nil {
		uc.Log.Error("clinicianUsecase.CreatePracticeInformation error parsing practitioner session",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, err
	}

	result := make([]responses.PracticeInformation, 0, len(req.PracticeInformation))
	uc.Log.Info("clinicianUsecase.CreatePracticeInformation processing practice information",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	for _, practiceInfo := range req.PracticeInformation {
		uc.Log.Info("clinicianUsecase.CreatePracticeInformation processing clinic",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingClinicIDKey, practiceInfo.ClinicID),
		)

		practitionerRoles, err := uc.fetchPractitionerRoles(ctx, session.PractitionerID, practiceInfo.ClinicID)
		if err != nil {
			uc.Log.Error("clinicianUsecase.CreatePracticeInformation error fetching practitioner roles",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.String(constvars.LoggingClinicIDKey, practiceInfo.ClinicID),
				zap.Error(err),
			)
			return nil, err
		}

		org, err := uc.OrganizationFhirClient.FindOrganizationByID(ctx, practiceInfo.ClinicID)
		if err != nil {
			uc.Log.Error("clinicianUsecase.CreatePracticeInformation error fetching organization",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.String(constvars.LoggingClinicIDKey, practiceInfo.ClinicID),
				zap.Error(err),
			)
			return nil, err
		}
		practiceInfo.ClinicName = org.Name
		uc.Log.Info("clinicianUsecase.CreatePracticeInformation organization fetched",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingClinicIDKey, practiceInfo.ClinicID),
			zap.String("clinic_name", org.Name),
		)

		practitionerRoleRequest := uc.buildPractitionerRoleRequestFromPracticeInformation(session.PractitionerID, practiceInfo, practitionerRoles)
		uc.Log.Info("clinicianUsecase.CreatePracticeInformation building practitioner role request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingClinicIDKey, practiceInfo.ClinicID),
		)

		savedPractitionerRole, err := uc.createOrUpdatePractitionerRole(ctx, practitionerRoleRequest)
		if err != nil {
			uc.Log.Error("clinicianUsecase.CreatePracticeInformation error creating/updating practitioner role",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.String(constvars.LoggingClinicIDKey, practiceInfo.ClinicID),
				zap.Error(err),
			)
			return nil, err
		}
		uc.Log.Info("clinicianUsecase.CreatePracticeInformation practitioner role processed",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingPractitionerRoleIDKey, savedPractitionerRole.ID),
		)

		practiceInfo.PractitionerRoleFullResourceID = utils.ParseSlashSeparatedToDashSeparated(
			fmt.Sprintf("%s/%s", constvars.ResourcePractitionerRole, savedPractitionerRole.ID),
		)
		uc.Log.Info("clinicianUsecase.CreatePracticeInformation updated practitioner role full resource ID",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String("practitioner_role_full_resource_id", practiceInfo.PractitionerRoleFullResourceID),
		)

		savedChargeItemDef, err := uc.createOrUpdateChargeItemDefinition(ctx, practiceInfo)
		if err != nil {
			uc.Log.Error("clinicianUsecase.CreatePracticeInformation error creating/updating charge item definition",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.String(constvars.LoggingClinicIDKey, practiceInfo.ClinicID),
				zap.Error(err),
			)
			return nil, err
		}
		uc.Log.Info("clinicianUsecase.CreatePracticeInformation charge item definition processed",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingChargeItemDefinitionIDKey, savedChargeItemDef.ID),
		)

		result = append(result, responses.PracticeInformation{
			ClinicID:    org.ID,
			ClinicName:  org.Name,
			Affiliation: org.Name,
			Specialties: utils.ExtractSpecialties(savedPractitionerRole.Specialty),
			PricePerSession: responses.PricePerSession{
				Value:    savedChargeItemDef.PropertyGroup[0].PriceComponent[0].Amount.Value,
				Currency: savedChargeItemDef.PropertyGroup[0].PriceComponent[0].Amount.Currency,
			},
		})
		uc.Log.Info("clinicianUsecase.CreatePracticeInformation appended practice information for clinic",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingClinicIDKey, practiceInfo.ClinicID),
		)
	}

	uc.Log.Info("clinicianUsecase.CreatePracticeInformation succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingPractitionerIDKey, session.PractitionerID),
		zap.Int(constvars.LoggingClinicCountKey, len(result)),
	)
	return result, nil
}

func (uc *clinicianUsecase) CreatePracticeAvailability(ctx context.Context, sessionData string, request *requests.CreatePracticeAvailability) ([]responses.PracticeAvailability, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	uc.Log.Info("clinicianUsecase.CreatePracticeAvailability called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	session, err := uc.SessionService.ParseSessionData(ctx, sessionData)
	if err != nil {
		uc.Log.Error("clinicianUsecase.CreatePracticeAvailability error parsing session data",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, err
	}

	if session.IsNotPractitioner() {
		uc.Log.Error("clinicianUsecase.CreatePracticeAvailability role mismatch: not a practitioner",
			zap.String(constvars.LoggingRequestIDKey, requestID),
		)
		return nil, exceptions.ErrNotMatchRoleType(nil)
	}

	var response []responses.PracticeAvailability

	for _, clinicID := range request.ClinicIDs {
		availableTimes := request.AvailableTimes[clinicID]
		uc.Log.Info("clinicianUsecase.CreatePracticeAvailability processing clinic",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingClinicIDKey, clinicID),
		)

		for _, availableTime := range availableTimes {
			practitionerRoles, err := uc.PractitionerRoleFhirClient.FindPractitionerRoleByPractitionerID(ctx, session.PractitionerID)
			if err != nil {
				uc.Log.Error("clinicianUsecase.CreatePracticeAvailability error fetching practitioner roles",
					zap.String(constvars.LoggingRequestIDKey, requestID),
					zap.String(constvars.LoggingClinicIDKey, clinicID),
					zap.Error(err),
				)
				return nil, err
			}
			hasConflict, err := uc.checkForTimeConflicts(practitionerRoles, availableTime)
			if err != nil {
				uc.Log.Error("clinicianUsecase.CreatePracticeAvailability error checking time conflicts",
					zap.String(constvars.LoggingRequestIDKey, requestID),
					zap.String(constvars.LoggingClinicIDKey, clinicID),
					zap.Error(err),
				)
				return nil, err
			}
			if hasConflict {
				customErr := fmt.Errorf("conflict detected for organization `%s` with available time %v", clinicID, availableTime)
				uc.Log.Error("clinicianUsecase.CreatePracticeAvailability time conflict detected",
					zap.String(constvars.LoggingRequestIDKey, requestID),
					zap.String(constvars.LoggingClinicIDKey, clinicID),
					zap.Error(customErr),
				)
				return nil, exceptions.ErrClientCustomMessage(customErr)
			}
		}

		practitionerRoles, err := uc.PractitionerRoleFhirClient.FindPractitionerRoleByPractitionerIDAndOrganizationID(ctx, session.PractitionerID, clinicID)
		if err != nil {
			uc.Log.Error("clinicianUsecase.CreatePracticeAvailability error fetching practitioner roles by clinic",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.String(constvars.LoggingPractitionerIDKey, session.PractitionerID),
				zap.String(constvars.LoggingClinicIDKey, clinicID),
				zap.Error(err),
			)
			return nil, err
		}

		practitionerRoleFhirRequest := uc.buildPractitionerRoleRequestForPracticeAvailability(practitionerRoles, availableTimes)
		uc.Log.Info("clinicianUsecase.CreatePracticeAvailability building practitioner role request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingClinicIDKey, clinicID),
		)

		practitionerRole, err := uc.PractitionerRoleFhirClient.UpdatePractitionerRole(ctx, practitionerRoleFhirRequest)
		if err != nil {
			uc.Log.Error("clinicianUsecase.CreatePracticeAvailability error updating practitioner role",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.String(constvars.LoggingClinicIDKey, clinicID),
				zap.Error(err),
			)
			return nil, err
		}

		schedules, err := uc.ScheduleFhirClient.FindScheduleByPractitionerRoleID(ctx, practitionerRole.ID)
		if err != nil {
			uc.Log.Error("clinicianUsecase.CreatePracticeAvailability error fetching schedule",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.String(constvars.LoggingPractitionerRoleIDKey, practitionerRole.ID),
				zap.Error(err),
			)
			return nil, err
		}

		if len(schedules) > 1 {
			uc.Log.Error("clinicianUsecase.CreatePracticeAvailability duplicate schedules found",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.String(constvars.LoggingPractitionerRoleIDKey, practitionerRole.ID),
			)
			return nil, exceptions.ErrGetFHIRResourceDuplicate(nil, constvars.ResourceSchedule)
		}

		if len(schedules) == 0 {
			scheduleFhirRequest := &fhir_dto.Schedule{
				ResourceType: constvars.ResourceSchedule,
				Actor: []fhir_dto.Reference{
					{
						Reference: fmt.Sprintf("%s/%s", constvars.ResourcePractitionerRole, practitionerRole.ID),
					},
					{
						Reference: fmt.Sprintf("%s/%s", constvars.ResourcePractitioner, session.PractitionerID),
					},
				},
				Comment: fmt.Sprintf("%s for %s (%s) %s (%s)",
					constvars.ResourceSchedule,
					constvars.ResourcePractitioner,
					session.PractitionerID,
					constvars.ResourcePractitionerRole,
					practitionerRole.ID,
				),
			}

			_, err = uc.ScheduleFhirClient.CreateSchedule(ctx, scheduleFhirRequest)
			if err != nil {
				uc.Log.Error("clinicianUsecase.CreatePracticeAvailability error creating schedule",
					zap.String(constvars.LoggingRequestIDKey, requestID),
					zap.String(constvars.LoggingPractitionerRoleIDKey, practitionerRole.ID),
					zap.Error(err),
				)
				return nil, err
			}
		}

		response = append(response, responses.PracticeAvailability{
			ClinicID:       clinicID,
			AvailableTimes: utils.ConvertToAvailableTimesResponse(practitionerRole.AvailableTime),
		})
	}

	uc.Log.Info("clinicianUsecase.CreatePracticeAvailability succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingPractitionerIDKey, session.PractitionerID),
		zap.Int(constvars.LoggingClinicCountKey, len(response)),
	)
	return response, nil
}

func (uc *clinicianUsecase) parseAndValidatePractitionerSession(ctx context.Context, sessionData string) (*models.Session, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	uc.Log.Info("clinicianUsecase.parseAndValidatePractitionerSession called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)
	session, err := uc.SessionService.ParseSessionData(ctx, sessionData)
	if err != nil {
		uc.Log.Error("clinicianUsecase.parseAndValidatePractitionerSession error parsing session data",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, err
	}
	if session.IsNotPractitioner() {
		uc.Log.Error("clinicianUsecase.parseAndValidatePractitionerSession role mismatch: not a practitioner",
			zap.String(constvars.LoggingRequestIDKey, requestID),
		)
		return nil, exceptions.ErrNotMatchRoleType(nil)
	}
	uc.Log.Info("clinicianUsecase.parseAndValidatePractitionerSession succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Any("session", session),
	)
	return session, nil
}

func (uc *clinicianUsecase) fetchPractitionerRoles(ctx context.Context, practitionerID, clinicID string) ([]fhir_dto.PractitionerRole, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	uc.Log.Info("clinicianUsecase.fetchPractitionerRoles called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingPractitionerIDKey, practitionerID),
		zap.String(constvars.LoggingClinicIDKey, clinicID),
	)

	roles, err := uc.PractitionerRoleFhirClient.FindPractitionerRoleByPractitionerIDAndOrganizationID(ctx, practitionerID, clinicID)
	if err != nil {
		uc.Log.Error("clinicianUsecase.fetchPractitionerRoles error fetching roles",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingPractitionerIDKey, practitionerID),
			zap.String(constvars.LoggingClinicIDKey, clinicID),
			zap.Error(err),
		)
		return nil, err
	}

	if len(roles) > 1 {
		uc.Log.Error("clinicianUsecase.fetchPractitionerRoles error: result not unique",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingPractitionerIDKey, practitionerID),
			zap.String(constvars.LoggingClinicIDKey, clinicID),
		)
		return nil, exceptions.ErrResultFetchedNotUniqueFhirResource(nil, constvars.ResourcePractitionerRole)
	}

	uc.Log.Info("clinicianUsecase.fetchPractitionerRoles succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingPractitionerIDKey, practitionerID),
		zap.String(constvars.LoggingClinicIDKey, clinicID),
	)
	return roles, nil
}

func (uc *clinicianUsecase) createOrUpdatePractitionerRole(ctx context.Context, practitionerRoleRequest *fhir_dto.PractitionerRole) (*fhir_dto.PractitionerRole, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	uc.Log.Info("clinicianUsecase.createOrUpdatePractitionerRole called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String("practitioner_role_request_id", practitionerRoleRequest.ID),
	)

	if practitionerRoleRequest.ID == "" {
		newRole, err := uc.PractitionerRoleFhirClient.CreatePractitionerRole(ctx, practitionerRoleRequest)
		if err != nil {
			uc.Log.Error("clinicianUsecase.createOrUpdatePractitionerRole error creating new practitioner role",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, err
		}
		uc.Log.Info("clinicianUsecase.createOrUpdatePractitionerRole created new practitioner role",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingPractitionerRoleIDKey, newRole.ID),
		)
		return newRole, nil
	}

	updatedRole, err := uc.PractitionerRoleFhirClient.UpdatePractitionerRole(ctx, practitionerRoleRequest)
	if err != nil {
		uc.Log.Error("clinicianUsecase.createOrUpdatePractitionerRole error updating practitioner role",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingPractitionerRoleIDKey, practitionerRoleRequest.ID),
			zap.Error(err),
		)
		return nil, err
	}

	uc.Log.Info("clinicianUsecase.createOrUpdatePractitionerRole updated practitioner role",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingPractitionerRoleIDKey, updatedRole.ID),
	)
	return updatedRole, nil
}

func (uc *clinicianUsecase) createOrUpdateChargeItemDefinition(ctx context.Context, practiceInfo requests.PracticeInformation) (*fhir_dto.ChargeItemDefinition, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	uc.Log.Info("clinicianUsecase.createOrUpdateChargeItemDefinition called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String("practitioner_role_full_resource_id", practiceInfo.PractitionerRoleFullResourceID),
	)

	foundCID, err := uc.ChargeItemDefinitionFhirClient.FindChargeItemDefinitionByID(ctx, practiceInfo.PractitionerRoleFullResourceID)
	if err != nil {
		uc.Log.Error("clinicianUsecase.createOrUpdateChargeItemDefinition error fetching charge item definition",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, err
	}

	if foundCID.ID == "" {
		cidReq := uc.buildChargeItemDefinition(practiceInfo)
		newCID, err := uc.ChargeItemDefinitionFhirClient.CreateChargeItemDefinition(ctx, cidReq)
		if err != nil {
			uc.Log.Error("clinicianUsecase.createOrUpdateChargeItemDefinition error creating charge item definition",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, err
		}
		uc.Log.Info("clinicianUsecase.createOrUpdateChargeItemDefinition created new charge item definition",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String("charge_item_definition_id", newCID.ID),
		)
		return newCID, nil
	}

	cidReq := uc.updateChargeItemDefinition(practiceInfo, foundCID)
	updatedCID, err := uc.ChargeItemDefinitionFhirClient.UpdateChargeItemDefinition(ctx, cidReq)
	if err != nil {
		uc.Log.Error("clinicianUsecase.createOrUpdateChargeItemDefinition error updating charge item definition",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String("charge_item_definition_id", foundCID.ID),
			zap.Error(err),
		)
		return nil, err
	}

	uc.Log.Info("clinicianUsecase.createOrUpdateChargeItemDefinition updated charge item definition",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String("charge_item_definition_id", updatedCID.ID),
	)
	return updatedCID, nil
}

func (uc *clinicianUsecase) buildPractitionerRoleRequestFromPracticeInformation(practitionerID string, practiceInformation requests.PracticeInformation, practitionerRoles []fhir_dto.PractitionerRole) *fhir_dto.PractitionerRole {
	practitionerRef := fhir_dto.Reference{
		Reference: fmt.Sprintf("%s/%s", constvars.ResourcePractitioner, practitionerID),
	}
	organizationRef := fhir_dto.Reference{
		Reference: fmt.Sprintf("%s/%s", constvars.ResourceOrganization, practiceInformation.ClinicID),
		Display:   practiceInformation.ClinicName,
	}

	request := &fhir_dto.PractitionerRole{
		ResourceType: constvars.ResourcePractitionerRole,
		Practitioner: practitionerRef,
		Organization: organizationRef,
		Active:       false,
		Specialty:    []fhir_dto.CodeableConcept{},
	}

	if len(practitionerRoles) == 1 && len(practitionerRoles[0].AvailableTime) > 0 {
		request.Active = true
		request.AvailableTime = practitionerRoles[0].AvailableTime
	}

	for _, specialty := range practiceInformation.Specialties {
		request.Specialty = append(request.Specialty, fhir_dto.CodeableConcept{
			Text: specialty,
		})
	}

	if len(practitionerRoles) == 1 {
		request.ID = practitionerRoles[0].ID
	}
	return request
}

func (uc *clinicianUsecase) buildChargeItemDefinition(practiceInfo requests.PracticeInformation) *fhir_dto.ChargeItemDefinition {
	return &fhir_dto.ChargeItemDefinition{
		ID:           practiceInfo.PractitionerRoleFullResourceID,
		ResourceType: constvars.ResourceChargeItemDefinition,
		Status:       constvars.FhirChargeItemDefinitionStatusActive,
		PropertyGroup: []fhir_dto.ChargeItemPropertyGroup{
			{
				PriceComponent: []fhir_dto.ChargeItemPriceComponent{
					{
						Type: constvars.FhirMonetaryComponentStatusBase,
						Amount: &fhir_dto.Money{
							Value:    practiceInfo.PricePerSession.Value,
							Currency: practiceInfo.PricePerSession.Currency,
						},
					},
				},
			},
		},
	}
}

func (uc *clinicianUsecase) updateChargeItemDefinition(practiceInfo requests.PracticeInformation, existingDef *fhir_dto.ChargeItemDefinition) *fhir_dto.ChargeItemDefinition {
	existingDef.PropertyGroup = []fhir_dto.ChargeItemPropertyGroup{
		{
			PriceComponent: []fhir_dto.ChargeItemPriceComponent{
				{
					Type: constvars.FhirMonetaryComponentStatusBase,
					Amount: &fhir_dto.Money{
						Value:    practiceInfo.PricePerSession.Value,
						Currency: practiceInfo.PricePerSession.Currency,
					},
				},
			},
		},
	}
	return existingDef
}

func (uc *clinicianUsecase) FindClinicsByClinicianID(ctx context.Context, request *requests.FindClinicianByClinicianID) ([]responses.ClinicianClinic, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	uc.Log.Info("clinicianUsecase.FindClinicsByClinicianID called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Any(constvars.LoggingRequestKey, request),
	)

	practitionerRoles, err := uc.PractitionerRoleFhirClient.FindPractitionerRoleByPractitionerIDAndName(ctx, request)
	if err != nil {
		uc.Log.Error("clinicianUsecase.FindClinicsByClinicianID error fetching practitioner roles",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, err
	}

	response, err := uc.findAndBuildClinicianCinicsResponseByPractitionerRoles(ctx, practitionerRoles)
	if err != nil {
		uc.Log.Error("clinicianUsecase.FindClinicsByClinicianID error building clinics response",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, err
	}

	uc.Log.Info("clinicianUsecase.FindClinicsByClinicianID succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Int(constvars.LoggingClinicCountKey, len(response)),
	)
	return response, nil
}

func (uc *clinicianUsecase) FindAvailability(ctx context.Context, request *requests.FindAvailability) (*responses.MonthlyAvailabilityResponse, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	uc.Log.Info("clinicianUsecase.FindAvailability called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Any(constvars.LoggingRequestKey, request),
	)

	yearInt, err := strconv.Atoi(request.Year)
	if err != nil {
		uc.Log.Error("clinicianUsecase.FindAvailability error parsing year",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrInvalidFormat(err, "year")
	}

	monthInt, err := strconv.Atoi(request.Month)
	if err != nil || monthInt < 1 || monthInt > 12 {
		uc.Log.Error("clinicianUsecase.FindAvailability error parsing month",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrInvalidFormat(err, "month")
	}

	startDate := time.Date(yearInt, time.Month(monthInt), 1, 0, 0, 0, 0, time.UTC)
	endDate := startDate.AddDate(0, 1, -1)

	practitionerRole, err := uc.PractitionerRoleFhirClient.FindPractitionerRoleByID(ctx, request.PractitionerRoleID)
	if err != nil {
		uc.Log.Error("clinicianUsecase.FindAvailability error fetching practitioner role",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingPractitionerRoleIDKey, request.PractitionerRoleID),
			zap.Error(err),
		)
		return nil, err
	}

	availableTimes := uc.findAvailableTimesForPractitionerRole(practitionerRole, startDate, endDate)
	uc.Log.Info("clinicianUsecase.FindAvailability available times computed",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	schedule, err := uc.ScheduleFhirClient.FindScheduleByPractitionerRoleID(ctx, practitionerRole.ID)
	if err != nil {
		uc.Log.Error("clinicianUsecase.FindAvailability error fetching schedule",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingPractitionerRoleIDKey, practitionerRole.ID),
			zap.Error(err),
		)
		return nil, err
	}

	if len(schedule) > 1 {
		uc.Log.Error("clinicianUsecase.FindAvailability error: duplicate schedules found",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingPractitionerRoleIDKey, practitionerRole.ID),
		)
		return nil, exceptions.ErrResultFetchedNotUniqueFhirResource(nil, constvars.ResourceSchedule)
	}

	if len(schedule) == 0 {
		uc.Log.Error("clinicianUsecase.FindAvailability error: no schedule found",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingPractitionerRoleIDKey, practitionerRole.ID),
		)
		return nil, exceptions.ErrNoDataFHIRResource(nil, constvars.ResourceSchedule)
	}

	slots, err := uc.SlotFhirClient.FindSlotByScheduleIDAndStatus(ctx, schedule[0].ID, constvars.FhirSlotStatusBusy)
	if err != nil {
		uc.Log.Error("clinicianUsecase.FindAvailability error fetching slots",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingScheduleIDKey, schedule[0].ID),
			zap.Error(err),
		)
		return nil, err
	}

	busySlots := uc.findBusySlots(slots)
	days := uc.generateDayAvailability(startDate, endDate, availableTimes, busySlots)

	uc.Log.Info("clinicianUsecase.FindAvailability succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)
	return &responses.MonthlyAvailabilityResponse{
		Year:  yearInt,
		Month: monthInt,
		Days:  days,
	}, nil
}

func (uc *clinicianUsecase) buildPractitionerRoleRequestForPracticeAvailability(practitionerRoles []fhir_dto.PractitionerRole, availableTimes []requests.AvailableTimeRequest) *fhir_dto.PractitionerRole {
	practitionerID := strings.Split(practitionerRoles[0].Practitioner.Reference, "/")[1]
	organizationID := strings.Split(practitionerRoles[0].Organization.Reference, "/")[1]

	practitionerReference := fhir_dto.Reference{
		Reference: fmt.Sprintf("%s/%s", constvars.ResourcePractitioner, practitionerID),
	}
	organizationReference := fhir_dto.Reference{
		Reference: fmt.Sprintf("%s/%s", constvars.ResourceOrganization, organizationID),
		Display:   practitionerRoles[0].Organization.Display,
	}

	request := &fhir_dto.PractitionerRole{
		ResourceType:  constvars.ResourcePractitionerRole,
		Practitioner:  practitionerReference,
		Organization:  organizationReference,
		Active:        true,
		Specialty:     []fhir_dto.CodeableConcept{},
		AvailableTime: utils.ConvertToModelAvailableTimes(availableTimes),
	}

	for _, specialty := range practitionerRoles[0].Specialty {
		request.Specialty = append(request.Specialty, fhir_dto.CodeableConcept{
			Text: specialty.Text,
		})
	}

	if len(practitionerRoles) == 1 {
		request.ID = practitionerRoles[0].ID
	}
	return request
}

func (uc *clinicianUsecase) generateDayAvailability(startDate, endDate time.Time, availableTimes, busySlots map[string][]string) []responses.DayAvailability {
	var days []responses.DayAvailability

	for date := startDate; date.Before(endDate) || date.Equal(endDate); date = date.AddDate(0, 0, 1) {
		dateStr := date.Format("2006-01-02")
		available := availableTimes[dateStr]
		unavailable := busySlots[dateStr]

		for _, timeSlot := range unavailable {
			utils.RemoveFromSlice(&available, timeSlot)
		}

		days = append(days, responses.DayAvailability{
			Date:             dateStr,
			AvailableTimes:   available,
			UnavailableTimes: unavailable,
		})
	}

	return days
}

func (uc *clinicianUsecase) findBusySlots(slots []fhir_dto.Slot) map[string][]string {
	busySlotsMap := make(map[string][]string)

	for _, slot := range slots {
		dateStrFormatted := slot.Start.Format("2006-01-02 15:04:05")
		dateStr := dateStrFormatted[:10]
		timeStr := dateStrFormatted[11:16]
		busySlotsMap[dateStr] = append(busySlotsMap[dateStr], timeStr)
	}

	return busySlotsMap
}

func (uc *clinicianUsecase) findAvailableTimesForPractitionerRole(practitionerRole *fhir_dto.PractitionerRole, start, end time.Time) map[string][]string {
	availableTimesMap := make(map[string][]string)

	for _, availableTime := range practitionerRole.AvailableTime {
		for date := start; date.Before(end) || date.Equal(end); date = date.AddDate(0, 0, 1) {
			dayOfWeek := date.Weekday().String()
			if utils.DaysContains(availableTime.DaysOfWeek, dayOfWeek) {
				dateStr := date.Format("2006-01-02")
				availableTimesMap[dateStr] = append(
					availableTimesMap[dateStr],
					utils.GenerateTimeSlots(availableTime.AvailableStartTime, availableTime.AvailableEndTime)...,
				)
			}
		}
	}

	return availableTimesMap
}

func (uc *clinicianUsecase) findAndBuildClinicianCinicsResponseByPractitionerRoles(ctx context.Context, practitionerRoles []fhir_dto.PractitionerRole) ([]responses.ClinicianClinic, error) {
	var response []responses.ClinicianClinic

	for _, practitionerRole := range practitionerRoles {
		parts := strings.Split(practitionerRole.Organization.Reference, "/")
		if len(parts) == 2 && parts[0] == "Organization" {
			var specialties []string

			organization, err := uc.OrganizationFhirClient.FindOrganizationByID(ctx, parts[1])
			if err != nil {
				uc.Log.Error("findAndBuildClinicianCinicsResponseByPractitionerRoles error fetching organization",
					zap.String("organization_id", parts[1]),
					zap.Error(err),
				)
				return nil, err
			}

			practitionerRoleCID := utils.ParseSlashSeparatedToDashSeparated(
				fmt.Sprintf("%s/%s", constvars.ResourcePractitionerRole, practitionerRole.ID),
			)
			chargeItemDefinition, err := uc.ChargeItemDefinitionFhirClient.FindChargeItemDefinitionByID(ctx, practitionerRoleCID)
			if err != nil {
				uc.Log.Error("findAndBuildClinicianCinicsResponseByPractitionerRoles error fetching charge item definition",
					zap.String(constvars.LoggingPractitionerRoleIDKey, practitionerRole.ID),
					zap.Error(err),
				)
				return nil, err
			}

			for _, specialty := range practitionerRole.Specialty {
				specialties = append(specialties, specialty.Text)
			}

			response = append(response, responses.ClinicianClinic{
				ClinicID:    organization.ID,
				ClinicName:  organization.Name,
				Specialties: specialties,
				PricePerSession: responses.PricePerSession{
					Value:    chargeItemDefinition.PropertyGroup[0].PriceComponent[0].Amount.Value,
					Currency: chargeItemDefinition.PropertyGroup[0].PriceComponent[0].Amount.Currency,
				},
			})
		}
	}

	return response, nil
}

func (uc *clinicianUsecase) checkForTimeConflicts(existingRoles []fhir_dto.PractitionerRole, availableTime requests.AvailableTimeRequest) (bool, error) {
	for _, role := range existingRoles {
		for _, existingTime := range role.AvailableTime {
			for _, day := range availableTime.DaysOfWeek {
				if utils.DaysContains(existingTime.DaysOfWeek, day) {
					existingStart, err := time.Parse(constvars.TimeFormatHoursMinutesSeconds, existingTime.AvailableStartTime)
					if err != nil {
						return false, exceptions.ErrCannotParseTime(err)
					}
					existingEnd, err := time.Parse(constvars.TimeFormatHoursMinutesSeconds, existingTime.AvailableEndTime)
					if err != nil {
						return false, exceptions.ErrCannotParseTime(err)
					}
					newStart, err := time.Parse(constvars.TimeFormatHoursMinutesSeconds, availableTime.AvailableStartTime)
					if err != nil {
						return false, exceptions.ErrCannotParseTime(err)
					}
					newEnd, err := time.Parse(constvars.TimeFormatHoursMinutesSeconds, availableTime.AvailableEndTime)
					if err != nil {
						return false, exceptions.ErrCannotParseTime(err)
					}

					if newStart.Before(existingEnd) && newEnd.After(existingStart) {
						return true, nil
					}
				}
			}
		}
	}
	return false, nil
}
