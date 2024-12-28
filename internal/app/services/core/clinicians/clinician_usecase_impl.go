package clinicians

import (
	"context"
	"errors"
	"fmt"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/dto/responses"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/fhir_dto"
	"konsulin-service/internal/pkg/utils"
	"strconv"
	"strings"
	"time"
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
}

func NewClinicianUsecase(
	practitionerFhirClient contracts.PractitionerFhirClient,
	practitionerRoleFhirClient contracts.PractitionerRoleFhirClient,
	organizationFhirClient contracts.OrganizationFhirClient,
	scheduleFhirClient contracts.ScheduleFhirClient,
	slotFhirClient contracts.SlotFhirClient,
	appointmentFhirClient contracts.AppointmentFhirClient,
	chargeItemDefinitionFhirClient contracts.ChargeItemDefinitionFhirClient,
	sessionService contracts.SessionService,
) contracts.ClinicianUsecase {
	return &clinicianUsecase{
		PractitionerFhirClient:         practitionerFhirClient,
		PractitionerRoleFhirClient:     practitionerRoleFhirClient,
		OrganizationFhirClient:         organizationFhirClient,
		ScheduleFhirClient:             scheduleFhirClient,
		SlotFhirClient:                 slotFhirClient,
		AppointmentFhirClient:          appointmentFhirClient,
		ChargeItemDefinitionFhirClient: chargeItemDefinitionFhirClient,
		SessionService:                 sessionService,
	}
}

func (uc *clinicianUsecase) DeleteClinicByID(ctx context.Context, sessionData, clinicID string) error {
	// Parse session data
	session, err := uc.SessionService.ParseSessionData(ctx, sessionData)
	if err != nil {
		return err
	}

	if session.IsNotPractitioner() {
		return exceptions.ErrNotMatchRoleType(nil)
	}

	practitionerRoles, err := uc.PractitionerRoleFhirClient.FindPractitionerRoleByPractitionerIDAndOrganizationID(ctx, session.PractitionerID, clinicID)
	if err != nil {
		return err
	}

	if len(practitionerRoles) > 1 {
		fhirError := fmt.Errorf("duplicate result for practitionerID: %s clinicID: %s", session.PractitionerID, clinicID)
		return exceptions.ErrGetFHIRResourceDuplicate(fhirError, constvars.ResourcePractitionerRole)
	}

	schedules, err := uc.ScheduleFhirClient.FindScheduleByPractitionerRoleID(ctx, practitionerRoles[0].ID)
	if err != nil {
		return fmt.Errorf("error fetching Schedule: %w", err)
	}

	if len(schedules) > 1 {
		fhirError := fmt.Errorf("duplicate result for practitionerRoleID: %s", practitionerRoles[0].ID)
		return exceptions.ErrGetFHIRResourceDuplicate(fhirError, constvars.ResourceSchedule)
	}

	slots, err := uc.SlotFhirClient.FindSlotByScheduleIDAndStatus(ctx, schedules[0].ID, constvars.FhirSlotStatusBusy)
	if err != nil {
		return err
	}

	if len(slots) > 0 {
		customErrMessage := errors.New("you can't delete this clinic from your practice, you still have on-goind appointmnets")
		return exceptions.ErrClientCustomMessage(customErrMessage)
	}
	return nil
}

func (uc *clinicianUsecase) CreatePracticeInformation(ctx context.Context, sessionData string, request *requests.CreatePracticeInformation) ([]responses.PracticeInformation, error) {
	// Parse session data
	session, err := uc.SessionService.ParseSessionData(ctx, sessionData)
	if err != nil {
		return nil, err
	}

	if session.IsNotPractitioner() {
		return nil, exceptions.ErrNotMatchRoleType(nil)
	}

	result := make([]responses.PracticeInformation, 0, len(request.PracticeInformation))

	for _, practiceInformation := range request.PracticeInformation {
		practitionerRoles, err := uc.PractitionerRoleFhirClient.FindPractitionerRoleByPractitionerIDAndOrganizationID(
			ctx,
			session.PractitionerID,
			practiceInformation.ClinicID,
		)
		if err != nil {
			return nil, err
		}

		if len(practitionerRoles) > 1 {
			return nil, exceptions.ErrResultFetchedNotUniqueFhirResource(nil, constvars.ResourcePractitionerRole)
		}

		practitionerRoleFhirRequest := uc.buildPractitionerRoleRequestFromPracticeInformation(
			session.PractitionerID,
			practiceInformation,
			practitionerRoles,
		)

		if practitionerRoleFhirRequest.ID == "" {
			practitionerRoleFhir, err := uc.PractitionerRoleFhirClient.CreatePractitionerRole(ctx, practitionerRoleFhirRequest)
			if err != nil {
				return nil, err
			}
			organization, err := uc.OrganizationFhirClient.FindOrganizationByID(ctx, practiceInformation.ClinicID)
			if err != nil {
				return nil, err
			}
			practiceInformation.PractitionerRoleFullResourceID = utils.ParseSlashSeparatedToDashSeparated(fmt.Sprintf("%s/%s", constvars.ResourcePractitionerRole, practitionerRoleFhir.ID))
			chargeItemDefinitionRequest := uc.buildChargeItemDefinition(practiceInformation)
			chargeItemDefinition, err := uc.ChargeItemDefinitionFhirClient.UpdateChargeItemDefinition(ctx, chargeItemDefinitionRequest)
			if err != nil {
				return nil, err
			}

			result = append(result, responses.PracticeInformation{
				ClinicID:    organization.ID,
				ClinicName:  organization.Name,
				Affiliation: organization.Name,
				Specialties: utils.ExtractSpecialties(practitionerRoleFhir.Specialty),
				PricePerSession: responses.PricePerSession{
					Value:    chargeItemDefinition.PropertyGroup[0].PriceComponent[0].Amount.Value,
					Currency: chargeItemDefinition.PropertyGroup[0].PriceComponent[0].Amount.Currency,
				},
			})
		} else if practitionerRoleFhirRequest.ID != "" {
			practitionerRoleFhir, err := uc.PractitionerRoleFhirClient.UpdatePractitionerRole(ctx, practitionerRoleFhirRequest)
			if err != nil {
				return nil, err
			}
			organization, err := uc.OrganizationFhirClient.FindOrganizationByID(ctx, practiceInformation.ClinicID)
			if err != nil {
				return nil, err
			}

			practiceInformation.PractitionerRoleFullResourceID = utils.ParseSlashSeparatedToDashSeparated(fmt.Sprintf("%s/%s", constvars.ResourcePractitionerRole, practitionerRoleFhir.ID))
			chargeItemDefinition, err := uc.ChargeItemDefinitionFhirClient.FindChargeItemDefinitionByID(ctx, practiceInformation.PractitionerRoleFullResourceID)
			if err != nil {
				return nil, err
			}

			if chargeItemDefinition.ID == "" {
				chargeItemDefinitionRequest := uc.buildChargeItemDefinition(practiceInformation)
				chargeItemDefinition, err = uc.ChargeItemDefinitionFhirClient.UpdateChargeItemDefinition(ctx, chargeItemDefinitionRequest)
				if err != nil {
					return nil, err
				}
			} else {
				chargeItemDefinitionRequest := uc.updateChargeItemDefinition(practiceInformation, chargeItemDefinition)
				chargeItemDefinition, err = uc.ChargeItemDefinitionFhirClient.UpdateChargeItemDefinition(ctx, chargeItemDefinitionRequest)
				if err != nil {
					return nil, err
				}
			}

			result = append(result, responses.PracticeInformation{
				ClinicID:    organization.ID,
				ClinicName:  organization.Name,
				Affiliation: organization.Name,
				Specialties: utils.ExtractSpecialties(practitionerRoleFhir.Specialty),
				PricePerSession: responses.PricePerSession{
					Value:    chargeItemDefinition.PropertyGroup[0].PriceComponent[0].Amount.Value,
					Currency: chargeItemDefinition.PropertyGroup[0].PriceComponent[0].Amount.Currency,
				},
			})
		}

	}

	return result, nil
}

func (uc *clinicianUsecase) CreatePracticeAvailability(ctx context.Context, sessionData string, request *requests.CreatePracticeAvailability) ([]responses.PracticeAvailability, error) {
	// Parse session data
	session, err := uc.SessionService.ParseSessionData(ctx, sessionData)
	if err != nil {
		return nil, err
	}

	if session.IsNotPractitioner() {
		return nil, exceptions.ErrNotMatchRoleType(nil)
	}

	response := make([]responses.PracticeAvailability, 0, len(request.ClinicIDs))
	// Iterate over the organization_ids and create a PractitionerRole for each
	for _, clinicID := range request.ClinicIDs {
		availableTimes := request.AvailableTimes[clinicID]

		// Check for time conflicts across all existing PractitionerRoles
		for _, availableTime := range availableTimes {
			practitionerRoles, err := uc.PractitionerRoleFhirClient.FindPractitionerRoleByPractitionerID(ctx, session.PractitionerID)
			if err != nil {
				return nil, err
			}
			hasConflict, err := uc.checkForTimeConflicts(practitionerRoles, availableTime)
			if err != nil {
				return nil, err
			}
			if hasConflict {
				customErr := fmt.Errorf("conflict detected for organization `%s` with available time %v", clinicID, availableTime)
				return nil, exceptions.ErrClientCustomMessage(customErr)
			}
		}

		practitionerRoles, err := uc.PractitionerRoleFhirClient.FindPractitionerRoleByPractitionerIDAndOrganizationID(ctx, session.PractitionerID, clinicID)
		if err != nil {
			return nil, err
		}

		practitionerRoleFhirRequest := uc.buildPractitionerRoleRequestForPracticeAvailability(practitionerRoles, availableTimes)

		// Create the PractitionerRole
		practitionerRole, err := uc.PractitionerRoleFhirClient.UpdatePractitionerRole(ctx, practitionerRoleFhirRequest)
		if err != nil {
			return nil, err
		}

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
			Comment: fmt.Sprintf("%s for %s (%s) %s (%s) ", constvars.ResourceSchedule, constvars.ResourcePractitioner, session.PractitionerID, constvars.ResourcePractitionerRole, practitionerRole.ID),
		}

		_, err = uc.ScheduleFhirClient.CreateSchedule(ctx, scheduleFhirRequest)
		if err != nil {
			return nil, err
		}

		response = append(response, responses.PracticeAvailability{
			ClinicID:       clinicID,
			AvailableTimes: utils.ConvertToAvailableTimesResponse(practitionerRole.AvailableTime),
		})
	}

	return response, nil
}

func (uc *clinicianUsecase) FindClinicsByClinicianID(ctx context.Context, request *requests.FindClinicianByClinicianID) ([]responses.ClinicianClinic, error) {
	practitionerRoles, err := uc.PractitionerRoleFhirClient.FindPractitionerRoleByPractitionerIDAndName(ctx, request)
	if err != nil {
		return nil, err
	}

	return uc.findAndBuildClinicianCinicsResponseByPractitionerRoles(ctx, practitionerRoles)
}

func (uc *clinicianUsecase) FindAvailability(ctx context.Context, request *requests.FindAvailability) (*responses.MonthlyAvailabilityResponse, error) {
	yearInt, err := strconv.Atoi(request.Year)
	if err != nil {
		return nil, exceptions.ErrInvalidFormat(err, "year")
	}

	monthInt, err := strconv.Atoi(request.Month)
	if err != nil || monthInt < 1 || monthInt > 12 {
		return nil, exceptions.ErrInvalidFormat(err, "month")
	}

	startDate := time.Date(yearInt, time.Month(monthInt), 1, 0, 0, 0, 0, time.UTC)
	endDate := startDate.AddDate(0, 1, -1)

	practitionerRole, err := uc.PractitionerRoleFhirClient.FindPractitionerRoleByID(ctx, request.PractitionerRoleID)
	if err != nil {
		return nil, err
	}

	availableTimes := uc.findAvailableTimesForPractitionerRole(practitionerRole, startDate, endDate)

	schedule, err := uc.ScheduleFhirClient.FindScheduleByPractitionerRoleID(ctx, practitionerRole.ID)
	if err != nil {
		return nil, err
	}

	if len(schedule) > 1 {
		return nil, exceptions.ErrResultFetchedNotUniqueFhirResource(nil, constvars.ResourceSchedule)
	}

	if len(schedule) == 0 {
		return nil, exceptions.ErrNoDataFHIRResource(nil, constvars.ResourceSchedule)
	}

	slots, err := uc.SlotFhirClient.FindSlotByScheduleIDAndStatus(ctx, schedule[0].ID, constvars.FhirSlotStatusBusy)
	if err != nil {
		return nil, err
	}

	busySlots := uc.findBusySlots(slots)

	days := uc.generateDayAvailability(startDate, endDate, availableTimes, busySlots)

	return &responses.MonthlyAvailabilityResponse{
		Year:  yearInt,
		Month: monthInt,
		Days:  days,
	}, nil

}

func (uc *clinicianUsecase) buildPractitionerRoleRequestFromPracticeInformation(practitionerID string, practiceInformation requests.PracticeInformation, practitionerRoles []fhir_dto.PractitionerRole) *fhir_dto.PractitionerRole {
	practitionerReference := fhir_dto.Reference{
		Reference: fmt.Sprintf("%s/%s", constvars.ResourcePractitioner, practitionerID),
	}
	organizationReference := fhir_dto.Reference{
		Reference: fmt.Sprintf("%s/%s", constvars.ResourceOrganization, practiceInformation.ClinicID),
	}

	request := &fhir_dto.PractitionerRole{
		ResourceType: constvars.ResourcePractitionerRole,
		Practitioner: practitionerReference,
		Organization: organizationReference,
		Active:       false,
		Specialty:    []fhir_dto.CodeableConcept{},
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

func (uc *clinicianUsecase) buildChargeItemDefinition(practiceInformation requests.PracticeInformation) *fhir_dto.ChargeItemDefinition {
	return &fhir_dto.ChargeItemDefinition{
		ID:           practiceInformation.PractitionerRoleFullResourceID,
		ResourceType: constvars.ResourceChargeItemDefinition,
		Status:       constvars.FhirChargeItemDefinitionStatusActive,
		PropertyGroup: []fhir_dto.ChargeItemPropertyGroup{
			{
				PriceComponent: []fhir_dto.ChargeItemPriceComponent{
					{
						Type: constvars.FhirMonetaryComponentStatusBase,
						Amount: &fhir_dto.Money{
							Value:    practiceInformation.PricePerSession.Value,
							Currency: practiceInformation.PricePerSession.Currency,
						},
					},
				},
			},
		},
	}
}

func (uc *clinicianUsecase) updateChargeItemDefinition(practiceInformation requests.PracticeInformation, chargeItemDefinition *fhir_dto.ChargeItemDefinition) *fhir_dto.ChargeItemDefinition {
	chargeItemDefinition.PropertyGroup = []fhir_dto.ChargeItemPropertyGroup{
		{
			PriceComponent: []fhir_dto.ChargeItemPriceComponent{
				{
					Type: constvars.FhirMonetaryComponentStatusBase,
					Amount: &fhir_dto.Money{
						Value:    practiceInformation.PricePerSession.Value,
						Currency: practiceInformation.PricePerSession.Currency,
					},
				},
			},
		},
	}
	return chargeItemDefinition
}

func (uc *clinicianUsecase) buildPractitionerRoleRequestForPracticeAvailability(practitionerRoles []fhir_dto.PractitionerRole, availableTimes []requests.AvailableTimeRequest) *fhir_dto.PractitionerRole {
	practitionerID := strings.Split(practitionerRoles[0].Practitioner.Reference, "/")[1]
	organizationID := strings.Split(practitionerRoles[0].Organization.Reference, "/")[1]

	practitionerReference := fhir_dto.Reference{
		Reference: fmt.Sprintf("%s/%s", constvars.ResourcePractitioner, practitionerID),
	}
	organizationReference := fhir_dto.Reference{
		Reference: fmt.Sprintf("%s/%s", constvars.ResourceOrganization, organizationID),
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
		availableTimes := availableTimes[dateStr]
		unavailableTimes := busySlots[dateStr]

		// Remove unavailable times from available times
		for _, time := range unavailableTimes {
			utils.RemoveFromSlice(&availableTimes, time)
		}

		days = append(days, responses.DayAvailability{
			Date:             dateStr,
			AvailableTimes:   availableTimes,
			UnavailableTimes: unavailableTimes,
		})
	}

	return days
}

func (uc *clinicianUsecase) findBusySlots(slots []fhir_dto.Slot) map[string][]string {
	// Initialize map to store busy slots
	busySlotsMap := make(map[string][]string)

	// Populate busy slots map
	for _, slot := range slots {
		dateStrFormatted := slot.Start.Format("02-01-2006 15:04:05")
		dateStr := dateStrFormatted[:10]
		timeStr := dateStrFormatted[11:16]
		busySlotsMap[dateStr] = append(busySlotsMap[dateStr], timeStr)
	}

	return busySlotsMap
}
func (uc *clinicianUsecase) findAvailableTimesForPractitionerRole(practitionerRole *fhir_dto.PractitionerRole, start, end time.Time) map[string][]string {
	// Initialize map to store available times
	availableTimesMap := make(map[string][]string)

	// Iterate over available times to populate the map
	for _, availableTime := range practitionerRole.AvailableTime {
		for date := start; date.Before(end) || date.Equal(end); date = date.AddDate(0, 0, 1) {
			dayOfWeek := date.Weekday().String()
			if utils.DaysContains(availableTime.DaysOfWeek, dayOfWeek) {
				dateStr := date.Format("2006-01-02")
				availableTimesMap[dateStr] = append(availableTimesMap[dateStr],
					utils.GenerateTimeSlots(availableTime.AvailableStartTime, availableTime.AvailableEndTime)...)
			}
		}
	}

	return availableTimesMap
}

func (uc *clinicianUsecase) findAndBuildClinicianCinicsResponseByPractitionerRoles(ctx context.Context, practitionerRoles []fhir_dto.PractitionerRole) ([]responses.ClinicianClinic, error) {
	response := make([]responses.ClinicianClinic, 0, len(practitionerRoles))

	for _, practitionerRole := range practitionerRoles {
		parts := strings.Split(practitionerRole.Organization.Reference, "/")
		if len(parts) == 2 && parts[0] == "Organization" {
			var specialties []string
			organization, err := uc.OrganizationFhirClient.FindOrganizationByID(ctx, parts[1])
			if err != nil {
				return nil, err
			}

			practitionerRoleChargeItemDefinitionID := utils.ParseSlashSeparatedToDashSeparated(fmt.Sprintf("%s/%s", constvars.ResourcePractitionerRole, practitionerRole.ID))
			chargeItemDefinition, err := uc.ChargeItemDefinitionFhirClient.FindChargeItemDefinitionByID(ctx, practitionerRoleChargeItemDefinitionID)
			if err != nil {
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
