package patients

import (
	"context"
	"fmt"
	"konsulin-service/internal/app/services/core/session"
	"konsulin-service/internal/app/services/fhir_spark/appointments"
	practitionerRoles "konsulin-service/internal/app/services/fhir_spark/practitioner_role"
	"konsulin-service/internal/app/services/fhir_spark/practitioners"
	"konsulin-service/internal/app/services/fhir_spark/schedules"
	"konsulin-service/internal/app/services/fhir_spark/slots"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/fhir_dto"
	"time"
)

type patientUsecase struct {
	PractitionerFhirClient     practitioners.PractitionerFhirClient
	PractitionerRoleFhirClient practitionerRoles.PractitionerRoleFhirClient
	ScheduleFhirClient         schedules.ScheduleFhirClient
	SlotFhirClient             slots.SlotFhirClient
	AppointmentFhirClient      appointments.AppointmentFhirClient
	SessionService             session.SessionService
}

func NewPatientUsecase(
	practitionerFhirClient practitioners.PractitionerFhirClient,
	practitionerRoleFhirClient practitionerRoles.PractitionerRoleFhirClient,
	scheduleFhirClient schedules.ScheduleFhirClient,
	slotFhirClient slots.SlotFhirClient,
	appointmentFhirClient appointments.AppointmentFhirClient,
	sessionService session.SessionService,
) PatientUsecase {
	return &patientUsecase{
		PractitionerFhirClient:     practitionerFhirClient,
		PractitionerRoleFhirClient: practitionerRoleFhirClient,
		ScheduleFhirClient:         scheduleFhirClient,
		SlotFhirClient:             slotFhirClient,
		AppointmentFhirClient:      appointmentFhirClient,
		SessionService:             sessionService,
	}
}

func (uc *patientUsecase) CreateAppointment(ctx context.Context, sessionData string, request *requests.CreateAppointmentRequest) (*fhir_dto.Appointment, error) {
	// Parse session data
	session, err := uc.SessionService.ParseSessionData(ctx, sessionData)
	if err != nil {
		return nil, err
	}

	if session.IsNotPatient() {
		return nil, exceptions.ErrNotMatchRoleType(nil)
	}

	appointmentStartTime, err := time.Parse("2006-01-02 15:04", fmt.Sprintf("%s %s", request.Date, request.Time))
	if err != nil {
		return nil, exceptions.ErrCannotParseTime(err)
	}

	var slotsToBook []fhir_dto.Reference
	var lastSlotBooked *fhir_dto.Slot
	for i := 0; i < request.NumberOfSessions; i++ {
		startTime := appointmentStartTime.Add(time.Duration(i) * 30 * time.Minute)
		endTime := startTime.Add(30 * time.Minute)

		slotFhirRequest := &fhir_dto.Slot{
			ResourceType: constvars.ResourceSlot,
			Schedule: fhir_dto.Reference{
				Reference: fmt.Sprintf("Schedule/%s", request.ScheduleID),
			},
			Status: constvars.FhirSlotStatusBusy,
			Start:  startTime,
			End:    endTime,
		}

		// Generate the slot on demand
		slot, err := uc.SlotFhirClient.CreateSlot(ctx, slotFhirRequest)
		if err != nil {
			return nil, err
		}

		slotsToBook = append(slotsToBook, fhir_dto.Reference{
			Reference: fmt.Sprintf("Slot/%s", slot.ID),
		})

		if i == (request.NumberOfSessions - 1) {
			lastSlotBooked = slot
		}
	}

	appointmentFhirRequest := &fhir_dto.Appointment{
		ResourceType: constvars.ResourceAppointment,
		Status:       constvars.FhirAppointmentStatusBooked,
		Start:        appointmentStartTime,
		End:          lastSlotBooked.End,
		Slot:         slotsToBook,
		Description:  request.ProblemBrief,
		Participant: []fhir_dto.AppointmentParticipant{
			{
				Actor: fhir_dto.Reference{
					Reference: fmt.Sprintf("%s/%s", constvars.ResourcePatient, session.PatientID),
				},
				Status: constvars.FhirParticipantStatusAccepted,
			},
			{
				Actor: fhir_dto.Reference{
					Reference: fmt.Sprintf("%s/%s", constvars.ResourcePractitioner, request.ClinicianID),
				},
				Status: constvars.FhirParticipantStatusAccepted,
			},
		},
	}

	savedAppointment, err := uc.AppointmentFhirClient.CreateAppointment(ctx, appointmentFhirRequest)
	if err != nil {
		return nil, err
	}

	return savedAppointment, nil
}
