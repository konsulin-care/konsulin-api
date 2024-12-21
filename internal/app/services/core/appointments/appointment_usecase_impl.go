package appointments

import (
	"context"
	"errors"
	"fmt"
	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/dto/responses"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/fhir_dto"
	"konsulin-service/internal/pkg/utils"
	"strings"
	"time"
)

type appointmentUsecase struct {
	ClinicianService       contracts.ClinicianUsecase
	AppointmentFhirClient  contracts.AppointmentFhirClient
	PatientFhirClient      contracts.PatientFhirClient
	PractitionerFhirClient contracts.PractitionerFhirClient
	SlotFhirClient         contracts.SlotFhirClient
	RedisRepository        contracts.RedisRepository
	SessionService         contracts.SessionService
	OyService              contracts.PaymentGatewayService
	InternalConfig         *config.InternalConfig
}

func NewAppointmentUsecase(
	clinicianService contracts.ClinicianUsecase,
	appointmentFhirClient contracts.AppointmentFhirClient,
	patientFhirClient contracts.PatientFhirClient,
	practitionerFhirClient contracts.PractitionerFhirClient,
	slotFhirClient contracts.SlotFhirClient,
	redisRepository contracts.RedisRepository,
	sessionService contracts.SessionService,
	oyService contracts.PaymentGatewayService,
	internalConfig *config.InternalConfig,
) contracts.AppointmentUsecase {
	return &appointmentUsecase{
		ClinicianService:       clinicianService,
		AppointmentFhirClient:  appointmentFhirClient,
		PatientFhirClient:      patientFhirClient,
		PractitionerFhirClient: practitionerFhirClient,
		SlotFhirClient:         slotFhirClient,
		RedisRepository:        redisRepository,
		SessionService:         sessionService,
		OyService:              oyService,
		InternalConfig:         internalConfig,
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

			requestPaymentStatus := &requests.PaymentRoutingStatus{
				PartnerTransactionID: eachAppointment.ID,
			}
			paymentStatusResponse, err := uc.OyService.CheckPaymentRoutingStatus(ctx, requestPaymentStatus)
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
				PaymentStatus:   paymentStatusResponse.PaymentStatus,
				PaymentLink:     paymentStatusResponse.PaymentInfo.PaymentCheckoutURL,
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

		requestPaymentStatus := &requests.PaymentRoutingStatus{
			PartnerTransactionID: eachAppointment.ID,
		}
		paymentStatusResponse, err := uc.OyService.CheckPaymentRoutingStatus(ctx, requestPaymentStatus)
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
			PaymentStatus:   paymentStatusResponse.PaymentStatus,
			PaymentLink:     paymentStatusResponse.PaymentInfo.PaymentCheckoutURL,
		})
	}

	return response, nil
}

func (uc *appointmentUsecase) CreateAppointment(ctx context.Context, sessionData string, request *requests.CreateAppointmentRequest) (*responses.CreateAppointment, error) {
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
		startTime := appointmentStartTime.Add(time.Duration(i) * time.Duration(uc.InternalConfig.App.SessionMultiplierInMinutes) * time.Minute)
		endTime := startTime.Add(time.Duration(uc.InternalConfig.App.SessionMultiplierInMinutes) * time.Minute)

		slotFhirRequest := &fhir_dto.Slot{
			ResourceType: constvars.ResourceSlot,
			Schedule: fhir_dto.Reference{
				Reference: fmt.Sprintf("%s/%s", constvars.ResourceSchedule, request.ScheduleID),
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
			Reference: fmt.Sprintf("%s/%s", constvars.ResourceSlot, slot.ID),
		})

		if i == (request.NumberOfSessions - 1) {
			lastSlotBooked = slot
		}
	}

	appointmentFhirRequest := &fhir_dto.Appointment{
		ResourceType: constvars.ResourceAppointment,
		Status:       constvars.FhirAppointmentStatusProposed,
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

	var totalPriceToBePaidByPatient int = request.NumberOfSessions * request.PricePerSession

	paymentRoutingRequest := &requests.PaymentRequest{
		UseLinkedAccount:      false,
		PartnerTransactionID:  savedAppointment.ID,
		NeedFrontend:          true,
		SenderEmail:           uc.InternalConfig.Konsulin.FinanceEmail,
		VADisplayName:         uc.InternalConfig.Konsulin.PaymentDisplayName,
		PaymentExpirationTime: uc.addAndGetTime(constvars.TIME_DIFFERENCE_JAKARTA, uc.InternalConfig.App.PaymentExpiredTimeInMinutes, 10),
		ReceiveAmount:         totalPriceToBePaidByPatient,
		ListEnablePaymentMethod: []string{
			requests.OY_PAYMENT_METHOD_BANK_TRANSFER,
		},
		ListEnableSOF: []string{
			requests.BANK_CODE_BNI,
			requests.BANK_CODE_BRI,
			requests.BANK_CODE_MANDIRI,
			requests.BANK_CODE_BTPN_JENIUS,
			requests.BANK_CODE_SYARIAH_INDONESIA,
		},
		PaymentRouting: []requests.PaymentRouting{
			{
				RecipientBank: uc.InternalConfig.Konsulin.BankCode,
				// RecipientAccount: uc.InternalConfig.Konsulin.BankAccountNumber,
				RecipientAccount: constvars.OY_MOCK_ACCOUNT_NUMBER_SUCCESS,
				RecipientEmail:   uc.InternalConfig.Konsulin.FinanceEmail,
				RecipientAmount:  totalPriceToBePaidByPatient,
			},
		},
	}

	paymentRequestDTO := utils.MapPaymentRequestToDTO(paymentRoutingRequest)

	paymentResponse, err := uc.OyService.CreatePaymentRouting(ctx, paymentRequestDTO)
	if err != nil {
		return nil, err
	}

	return &responses.CreateAppointment{
		Status:               paymentResponse.Status.Message,
		TransactionID:        paymentResponse.TrxID,
		PartnerTransactionID: paymentResponse.PartnerTrxID,
		PaymentLink:          paymentResponse.PaymentInfo.PaymentCheckoutURL,
	}, nil
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

func (uc *appointmentUsecase) addAndGetTime(hoursToAdd, minutesToAdd, secondsToAdd int) string {
	currentTime := time.Now().UTC()

	newTime := currentTime.Add(
		time.Duration(hoursToAdd)*time.Hour +
			time.Duration(minutesToAdd)*time.Minute +
			time.Duration(secondsToAdd)*time.Second)

	return newTime.Format("2006-01-02 15:04:05")
}
