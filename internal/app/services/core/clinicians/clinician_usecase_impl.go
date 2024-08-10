package clinicians

import (
	"context"
	"fmt"
	"konsulin-service/internal/app/services/core/session"
	"konsulin-service/internal/app/services/fhir_spark/appointments"
	practitionerRoles "konsulin-service/internal/app/services/fhir_spark/practitioner_role"
	"konsulin-service/internal/app/services/fhir_spark/practitioners"
	"konsulin-service/internal/app/services/fhir_spark/schedules"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/dto/responses"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/utils"
)

type clinicianUsecase struct {
	PractitionerFhirClient     practitioners.PractitionerFhirClient
	PractitionerRoleFhirClient practitionerRoles.PractitionerRoleFhirClient
	ScheduleFhirClient         schedules.ScheduleFhirClient
	AppointmentFhirClient      appointments.AppointmentFhirClient
	SessionService             session.SessionService
}

func NewClinicianUsecase(
	practitionerFhirClient practitioners.PractitionerFhirClient,
	practitionerRoleFhirClient practitionerRoles.PractitionerRoleFhirClient,
	scheduleFhirClient schedules.ScheduleFhirClient,
	appointmentFhirClient appointments.AppointmentFhirClient,
	sessionService session.SessionService,
) ClinicianUsecase {
	return &clinicianUsecase{
		PractitionerFhirClient:     practitionerFhirClient,
		PractitionerRoleFhirClient: practitionerRoleFhirClient,
		ScheduleFhirClient:         scheduleFhirClient,
		AppointmentFhirClient:      appointmentFhirClient,
		SessionService:             sessionService,
	}
}

func (uc *clinicianUsecase) FindClinicianSummaryByID(ctx context.Context, practitionerID string) (*responses.ClinicianSummary, error) {
	practitioner, err := uc.PractitionerFhirClient.FindPractitionerByID(ctx, practitionerID)
	if err != nil {
		return nil, err
	}

	practitionerRoles, err := uc.PractitionerRoleFhirClient.FindPractitionerRoleByPractitionerID(ctx, practitioner.ID)
	if err != nil {
		return nil, err
	}

	var availableTimes []responses.AvailableTime
	var specialties []string

	for _, practitionerRole := range practitionerRoles {
		availableTimes = append(availableTimes, practitionerRole.AvailableTime...)
		specialties = append(specialties, utils.ExtractSpecialties(practitionerRole.Specialty)...)
	}

	practiceInformation := responses.PracticeInformation{
		Affiliation: "Konsulin",
		Experience:  "2 Years",
		Fee:         "250.000/session",
	}

	response := &responses.ClinicianSummary{
		PractitionerID:      practitioner.ID,
		Name:                utils.GetFullName(practitioner.Name),
		Affiliation:         "Konsulin",
		PracticeInformation: practiceInformation,
		Specialties:         specialties,
		Availability:        availableTimes,
	}

	return response, nil
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

	practitionerRole, err := uc.PractitionerRoleFhirClient.FindPractitionerRoleByPractitionerIDAndOrganizationID(ctx, session.PractitionerID, clinicID)
	if err != nil {
		return err
	}

	fmt.Println(practitionerRole.ID)

	err = uc.PractitionerRoleFhirClient.DeletePractitionerRoleByID(ctx, practitionerRole.ID)
	if err != nil {
		return err
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
