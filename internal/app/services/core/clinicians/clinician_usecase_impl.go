package clinicians

import (
	"context"
	"errors"
	"fmt"
	"konsulin-service/internal/app/services/core/session"
	"konsulin-service/internal/app/services/fhir_spark/appointments"
	"konsulin-service/internal/app/services/fhir_spark/organizations"
	practitionerRoles "konsulin-service/internal/app/services/fhir_spark/practitioner_role"
	"konsulin-service/internal/app/services/fhir_spark/practitioners"
	"konsulin-service/internal/app/services/fhir_spark/schedules"
	"konsulin-service/internal/app/services/fhir_spark/slots"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/dto/responses"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/utils"
	"strconv"
	"strings"
	"time"
)

type clinicianUsecase struct {
	PractitionerFhirClient     practitioners.PractitionerFhirClient
	PractitionerRoleFhirClient practitionerRoles.PractitionerRoleFhirClient
	OrganizationFhirClient     organizations.OrganizationFhirClient
	ScheduleFhirClient         schedules.ScheduleFhirClient
	SlotFhirClient             slots.SlotFhirClient
	AppointmentFhirClient      appointments.AppointmentFhirClient
	SessionService             session.SessionService
}

func NewClinicianUsecase(
	practitionerFhirClient practitioners.PractitionerFhirClient,
	practitionerRoleFhirClient practitionerRoles.PractitionerRoleFhirClient,
	organizationFhirClient organizations.OrganizationFhirClient,
	scheduleFhirClient schedules.ScheduleFhirClient,
	slotFhirClient slots.SlotFhirClient,
	appointmentFhirClient appointments.AppointmentFhirClient,
	sessionService session.SessionService,
) ClinicianUsecase {
	return &clinicianUsecase{
		PractitionerFhirClient:     practitionerFhirClient,
		PractitionerRoleFhirClient: practitionerRoleFhirClient,
		OrganizationFhirClient:     organizationFhirClient,
		ScheduleFhirClient:         scheduleFhirClient,
		SlotFhirClient:             slotFhirClient,
		AppointmentFhirClient:      appointmentFhirClient,
		SessionService:             sessionService,
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

func (uc *clinicianUsecase) CreateAvailibilityTime(ctx context.Context, sessionData string, request *requests.AvailableTime) error {
	// Parse session data
	session, err := uc.SessionService.ParseSessionData(ctx, sessionData)
	if err != nil {
		return err
	}

	if session.IsNotPractitioner() {
		return exceptions.ErrNotMatchRoleType(nil)
	}

	// _, err = uc.PractitionerRoleFhirClient.F()

	return nil
}

func (uc *clinicianUsecase) CreateAppointment(ctx context.Context, sessionData string, request *requests.CreateAppointmentRequest) error {
	// // Parse session data
	// session, err := uc.SessionService.ParseSessionData(ctx, sessionData)
	// if err != nil {
	// 	return err
	// }

	// // Parse the date and time
	// appointmentStartTime, err := time.Parse("2006-01-02 15:04", fmt.Sprintf("%s %s", request.Date, request.Time))
	// if err != nil {
	// 	return exceptions.ErrCannotParseDate(err)
	// }

	// var appointmentsToBook []*requests.Appointment
	// for i := 0; i < request.NumberOfSessions; i++ {
	// 	startTime := appointmentStartTime.Add(time.Duration(i) * 30 * time.Minute)
	// 	endTime := startTime.Add(30 * time.Minute)

	// 	// Check if the appointment is available
	// 	isAvailable, err := uc.practitionerRoleRepo.CheckClinicianAvailability(request.ClinicianId, startTime, endTime)
	// 	if err != nil {
	// 		return err
	// 	}
	// 	if !isAvailable {
	// 		return exceptions.ErrClientCustomMessage(fmt.Errorf("clinician is not available from %s to %s", startTime.Format("15:04"), endTime.Format("15:04")))
	// 	}

	// 	// Generate the appointment on demand
	// 	appointment, err := uc.practitionerRoleRepo.GenerateAppointmentOnDemand(request.ClinicianId, startTime.Format("2006-01-02"), startTime.Format("15:04"), endTime)
	// 	if err != nil {
	// 		return exceptions.ErrClientCustomMessage(fmt.Errorf("error generating appointment: %w", err))
	// 	}

	// 	appointmentsToBook = append(appointmentsToBook, appointment)
	// }

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
			response, err := uc.PractitionerRoleFhirClient.CreatePractitionerRole(ctx, practitionerRoleFhirRequest)
			if err != nil {
				return nil, err
			}
			organization, err := uc.OrganizationFhirClient.FindOrganizationByID(ctx, practiceInformation.ClinicID)
			if err != nil {
				return nil, err
			}

			result = append(result, responses.PracticeInformation{
				ClinicID:    organization.ID,
				ClinicName:  organization.Name,
				Affiliation: organization.Name,
				Specialties: utils.ExtractSpecialties(response.Specialty),
				PricePerSession: responses.PricePerSession{
					Value:    response.Extension[0].ValueMoney.Value,
					Currency: response.Extension[0].ValueMoney.Currency,
				},
			})
		} else if practitionerRoleFhirRequest.ID != "" {
			response, err := uc.PractitionerRoleFhirClient.UpdatePractitionerRole(ctx, practitionerRoleFhirRequest)
			if err != nil {
				return nil, err
			}
			organization, err := uc.OrganizationFhirClient.FindOrganizationByID(ctx, practiceInformation.ClinicID)
			if err != nil {
				return nil, err
			}

			result = append(result, responses.PracticeInformation{
				ClinicID:    organization.ID,
				ClinicName:  organization.Name,
				Affiliation: organization.Name,
				Specialties: utils.ExtractSpecialties(response.Specialty),
				PricePerSession: responses.PricePerSession{
					Value:    response.Extension[0].ValueMoney.Value,
					Currency: response.Extension[0].ValueMoney.Currency,
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
			hasConflict, err := uc.checkForTimeConflicts(ctx, practitionerRoles, availableTime)
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

		scheduleFhirRequest := &requests.Schedule{
			ResourceType: constvars.ResourceSchedule,
			Actor: []requests.Reference{
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

func (uc *clinicianUsecase) buildPractitionerRoleRequestFromPracticeInformation(practitionerID string, practiceInformation requests.PracticeInformation, practitionerRoles []responses.PractitionerRole) *requests.PractitionerRole {
	practitionerReference := requests.Reference{
		Reference: fmt.Sprintf("%s/%s", constvars.ResourcePractitioner, practitionerID),
	}
	organizationReference := requests.Reference{
		Reference: fmt.Sprintf("%s/%s", constvars.ResourceOrganization, practiceInformation.ClinicID),
	}

	extension := requests.Extension{
		Url: "http://hl7.org/fhir/StructureDefinition/Money",
		ValueMoney: requests.Money{
			Value:    practiceInformation.PricePerSession.Value,
			Currency: practiceInformation.PricePerSession.Currency,
		},
	}

	request := &requests.PractitionerRole{
		ResourceType: constvars.ResourcePractitionerRole,
		Practitioner: practitionerReference,
		Organization: organizationReference,
		Active:       false,
		Extension: []requests.Extension{
			extension,
		},
		Specialty: []requests.CodeableConcept{},
	}

	for _, specialty := range practiceInformation.Specialties {
		request.Specialty = append(request.Specialty, requests.CodeableConcept{
			Text: specialty,
		})
	}

	if len(practitionerRoles) == 1 {
		request.ID = practitionerRoles[0].ID
	}

	return request

}

func (uc *clinicianUsecase) buildPractitionerRoleRequestForPracticeAvailability(practitionerRoles []responses.PractitionerRole, availableTimes []requests.AvailableTimeRequest) *requests.PractitionerRole {
	practitionerID := strings.Split(practitionerRoles[0].Practitioner.Reference, "/")[1]
	organizationID := strings.Split(practitionerRoles[0].Organization.Reference, "/")[1]

	practitionerReference := requests.Reference{
		Reference: fmt.Sprintf("%s/%s", constvars.ResourcePractitioner, practitionerID),
	}
	organizationReference := requests.Reference{
		Reference: fmt.Sprintf("%s/%s", constvars.ResourceOrganization, organizationID),
	}

	extension := requests.Extension{
		Url: "http://hl7.org/fhir/StructureDefinition/Money",
		ValueMoney: requests.Money{
			Value:    practitionerRoles[0].Extension[0].ValueMoney.Value,
			Currency: practitionerRoles[0].Extension[0].ValueMoney.Currency,
		},
	}

	request := &requests.PractitionerRole{
		ResourceType: constvars.ResourcePractitionerRole,
		Practitioner: practitionerReference,
		Organization: organizationReference,
		Active:       true,
		Extension: []requests.Extension{
			extension,
		},
		Specialty:     []requests.CodeableConcept{},
		AvailableTime: utils.ConvertToModelAvailableTimes(availableTimes),
	}

	for _, specialty := range practitionerRoles[0].Specialty {
		request.Specialty = append(request.Specialty, requests.CodeableConcept{
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

func (uc *clinicianUsecase) findBusySlots(slots []responses.Slot) map[string][]string {
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
func (uc *clinicianUsecase) findAvailableTimesForPractitionerRole(practitionerRole *responses.PractitionerRole, start, end time.Time) map[string][]string {
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

func (uc *clinicianUsecase) findAndBuildClinicianCinicsResponseByPractitionerRoles(ctx context.Context, practitionerRoles []responses.PractitionerRole) ([]responses.ClinicianClinic, error) {
	response := make([]responses.ClinicianClinic, 0, len(practitionerRoles))

	for _, practitionerRole := range practitionerRoles {
		parts := strings.Split(practitionerRole.Organization.Reference, "/")
		if len(parts) == 2 && parts[0] == "Organization" {
			var specialties []string
			organization, err := uc.OrganizationFhirClient.FindOrganizationByID(ctx, parts[1])
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
					Value:    practitionerRole.Extension[0].ValueMoney.Value,
					Currency: practitionerRole.Extension[0].ValueMoney.Currency,
				},
			})
		}
	}

	return response, nil
}

func (uc *clinicianUsecase) checkForTimeConflicts(ctx context.Context, existingRoles []responses.PractitionerRole, availableTime requests.AvailableTimeRequest) (bool, error) {
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
