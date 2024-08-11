package clinicians

import (
	"context"
	"errors"
	"fmt"
	"konsulin-service/internal/app/services/core/session"
	"konsulin-service/internal/app/services/fhir_spark/appointments"
	practitionerRoles "konsulin-service/internal/app/services/fhir_spark/practitioner_role"
	"konsulin-service/internal/app/services/fhir_spark/practitioners"
	"konsulin-service/internal/app/services/fhir_spark/schedules"
	"konsulin-service/internal/app/services/fhir_spark/slots"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/dto/responses"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/utils"
	"time"
)

type clinicianUsecase struct {
	PractitionerFhirClient     practitioners.PractitionerFhirClient
	PractitionerRoleFhirClient practitionerRoles.PractitionerRoleFhirClient
	ScheduleFhirClient         schedules.ScheduleFhirClient
	SlotFhirClient             slots.SlotFhirClient
	AppointmentFhirClient      appointments.AppointmentFhirClient
	SessionService             session.SessionService
}

func NewClinicianUsecase(
	practitionerFhirClient practitioners.PractitionerFhirClient,
	practitionerRoleFhirClient practitionerRoles.PractitionerRoleFhirClient,
	scheduleFhirClient schedules.ScheduleFhirClient,
	slotFhirClient slots.SlotFhirClient,
	appointmentFhirClient appointments.AppointmentFhirClient,
	sessionService session.SessionService,
) ClinicianUsecase {
	return &clinicianUsecase{
		PractitionerFhirClient:     practitionerFhirClient,
		PractitionerRoleFhirClient: practitionerRoleFhirClient,
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

func (uc *clinicianUsecase) CreateClinics(ctx context.Context, sessionData string, request *requests.ClinicianCreateClinics) error {
	// Parse session data
	session, err := uc.SessionService.ParseSessionData(ctx, sessionData)
	if err != nil {
		return err
	}

	if session.IsNotPractitioner() {
		return exceptions.ErrNotMatchRoleType(nil)
	}

	// Build the bundle PractitionerRoles resources
	practitionerRoleBundleRequests := utils.BuildPractitionerRolesBundleRequestByPractitionerID(session.PractitionerID, request.ClinicIDs)

	// Bulk create the PractitionerRoles for the clinician
	err = uc.PractitionerRoleFhirClient.CreatePractitionerRoles(ctx, practitionerRoleBundleRequests)
	if err != nil {
		return err
	}

	return nil
}

func (uc *clinicianUsecase) CreateClinicsAvailability(ctx context.Context, sessionData string, request *requests.CreateClinicsAvailability) error {
	// Parse session data
	session, err := uc.SessionService.ParseSessionData(ctx, sessionData)
	if err != nil {
		return err
	}

	if session.IsNotPractitioner() {
		return exceptions.ErrNotMatchRoleType(nil)
	}

	// Iterate over the organization_ids and create a PractitionerRole for each
	for _, clinicID := range request.ClinicIDs {
		availableTimes := request.AvailableTimes[clinicID]

		// Check for time conflicts across all existing PractitionerRoles
		for _, availableTime := range availableTimes {
			practitionerRoles, err := uc.PractitionerRoleFhirClient.FindPractitionerRoleByPractitionerID(ctx, session.PractitionerID)
			if err != nil {
				return err
			}
			hasConflict, err := uc.checkForTimeConflicts(practitionerRoles, availableTime)
			if err != nil {
				return fmt.Errorf("error checking time conflicts: %w", err)
			}
			if hasConflict {
				return fmt.Errorf("conflict detected for organization %s with available time %v", clinicID, availableTime)
			}
		}

		practitionerRoleFhirRequest := &requests.PractitionerRole{
			ResourceType: constvars.ResourcePractitionerRole,
			Practitioner: requests.Reference{
				Reference: fmt.Sprintf("Practitioner/%s", session.PractitionerID),
			},
			Organization: requests.Reference{
				Reference: fmt.Sprintf("Organization/%s", clinicID),
			},
			AvailableTime: utils.ConvertToModelAvailableTimes(availableTimes),
		}

		// Create the PractitionerRole
		practitionerRole, err := uc.PractitionerRoleFhirClient.CreatePractitionerRole(ctx, practitionerRoleFhirRequest)
		if err != nil {
			return err
		}

		scheduleFhirRequest := &requests.ScheduleFhir{
			ResourceType: constvars.ResourceSchedule,
			Actor: []requests.Reference{
				{
					Reference: fmt.Sprintf("PractitionerRole/%s", practitionerRole.ID),
				},
				{
					Reference: fmt.Sprintf("Practitioner/%s", session.PractitionerID),
				},
			},
			Comment: fmt.Sprintf("Schedule for Practitioner (%s) PractitionerRole (%s) ", session.PractitionerID, practitionerRole.ID),
		}

		_, err = uc.ScheduleFhirClient.CreateSchedule(ctx, scheduleFhirRequest)
		if err != nil {
			return err
		}

	}

	return nil
}

func (u *clinicianUsecase) checkForTimeConflicts(existingRoles []responses.PractitionerRole, availableTime requests.AvailableTimeRequest) (bool, error) {
	for _, role := range existingRoles {
		for _, existingTime := range role.AvailableTime {
			for _, day := range availableTime.DaysOfWeek {
				if utils.Contains(existingTime.DaysOfWeek, day) {
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
