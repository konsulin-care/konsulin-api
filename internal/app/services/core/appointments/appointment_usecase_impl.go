package appointments

import (
	"context"
	"errors"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/dto/responses"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/fhir_dto"
	"konsulin-service/internal/pkg/utils"
	"strings"
)

type appointmentUsecase struct {
	AppointmentFhirClient  contracts.AppointmentFhirClient
	PatientFhirClient      contracts.PatientFhirClient
	PractitionerFhirClient contracts.PractitionerFhirClient
	RedisRepository        contracts.RedisRepository
	SessionService         contracts.SessionService
}

func NewAppointmentUsecase(
	appointmentFhirClient contracts.AppointmentFhirClient,
	patientFhirClient contracts.PatientFhirClient,
	practitionerFhirClient contracts.PractitionerFhirClient,
	redisRepository contracts.RedisRepository,
	sessionService contracts.SessionService,
) contracts.AppointmentUsecase {
	return &appointmentUsecase{
		AppointmentFhirClient: appointmentFhirClient,
		PatientFhirClient:     patientFhirClient,
		RedisRepository:       redisRepository,
		SessionService:        sessionService,
	}
}

func (uc *appointmentUsecase) FindAll(ctx context.Context, sessionData string, queryParamsRequest *requests.QueryParams) ([]responses.Appointment, error) {
	session, err := uc.SessionService.ParseSessionData(ctx, sessionData)
	if err != nil {
		return nil, err
	}

	if session.IsPatient() {
		queryParamsRequest.PatientID = session.PatientID

		appointments, err := uc.AppointmentFhirClient.FindAll(ctx, queryParamsRequest)
		if err != nil {
			return nil, err
		}

		response := make([]responses.Appointment, 0, len(appointments))
		for _, eachAppointment := range appointments {
			patientID, err := uc.FindPatientIDFromFhirAppointment(ctx, eachAppointment)
			if err != nil {
				return nil, err
			}

			patient, err := uc.PatientFhirClient.FindPatientByID(ctx, patientID)
			if err != nil {
				return nil, err
			}

			response = append(response, responses.Appointment{
				ID:              eachAppointment.ID,
				Status:          eachAppointment.Status,
				AppointmentTime: eachAppointment.Start,
				Description:     eachAppointment.Description,
				MinutesDuration: eachAppointment.MinutesDuration,
				PatientID:       patientID,
				PatientName:     utils.GetFullName(patient.Name),
			})
		}

		return response, nil
	}

	queryParamsRequest.PractitionerID = session.PractitionerID

	appointments, err := uc.AppointmentFhirClient.FindAll(ctx, queryParamsRequest)
	if err != nil {
		return nil, err
	}

	response := make([]responses.Appointment, 0, len(appointments))

	for _, eachAppointment := range appointments {
		practitionerID, err := uc.FindPractitionerIDFromFhirAppointment(ctx, eachAppointment)
		if err != nil {
			return nil, err
		}

		practitioner, err := uc.PractitionerFhirClient.FindPractitionerByID(ctx, practitionerID)
		if err != nil {
			return nil, err
		}

		response = append(response, responses.Appointment{
			ID:              eachAppointment.ID,
			Status:          eachAppointment.Status,
			AppointmentTime: eachAppointment.Start,
			Description:     eachAppointment.Description,
			MinutesDuration: eachAppointment.MinutesDuration,
			ClinicianID:     practitionerID,
			ClinicianName:   utils.GetFullName(practitioner.Name),
		})
	}

	return response, nil
}

func (uc *appointmentUsecase) FindPatientIDFromFhirAppointment(ctx context.Context, request fhir_dto.Appointment) (string, error) {
	for _, participant := range request.Participant {
		if strings.Contains(participant.Actor.Reference, "Patient/") {
			parts := strings.Split(participant.Actor.Reference, "/")
			if len(parts) > 1 {
				return parts[1], nil
			}
		}
	}
	errResponse := errors.New("patient ID not found in appointment")
	return "", exceptions.ErrServerProcess(errResponse)
}

func (uc *appointmentUsecase) FindPractitionerIDFromFhirAppointment(ctx context.Context, request fhir_dto.Appointment) (string, error) {
	for _, participant := range request.Participant {
		if strings.Contains(participant.Actor.Reference, "Practitioner/") {
			parts := strings.Split(participant.Actor.Reference, "/")
			if len(parts) > 1 {
				return parts[1], nil
			}
		}
	}
	errResponse := errors.New("practitioner ID not found in appointment")
	return "", exceptions.ErrServerProcess(errResponse)
}
