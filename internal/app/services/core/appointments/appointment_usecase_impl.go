package appointments

import (
	"context"
	"fmt"
	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/app/models"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/dto/responses"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/fhir_dto"
	"konsulin-service/internal/pkg/utils"
	"time"
)

type appointmentUsecase struct {
	TransactionRepository  contracts.TransactionRepository
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
	transactionRepository contracts.TransactionRepository,
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
		TransactionRepository:  transactionRepository,
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
	} else if session.IsPractitioner() {
		queryParamsRequest.PractitionerID = session.PractitionerID
	}

	appointments, err := uc.AppointmentFhirClient.FindAll(ctx, queryParamsRequest)
	if err != nil {
		return nil, err
	}

	response := make([]responses.Appointment, 0, len(appointments))
	for _, eachAppointment := range appointments {
		appointmentResp, err := uc.buildAppointmentResponse(ctx, eachAppointment)
		if err != nil {
			return nil, err
		}
		response = append(response, appointmentResp)
	}

	return response, nil
}

func (uc *appointmentUsecase) buildAppointmentResponse(ctx context.Context, fhirAppointment fhir_dto.Appointment) (responses.Appointment, error) {
	patientID, err := utils.FindPatientIDFromFhirAppointment(ctx, fhirAppointment)
	if err != nil {
		return responses.Appointment{}, err
	}

	practitionerID, err := utils.FindPractitionerIDFromFhirAppointment(ctx, fhirAppointment)
	if err != nil {
		return responses.Appointment{}, err
	}

	patient, err := uc.PatientFhirClient.FindPatientByID(ctx, patientID)
	if err != nil {
		return responses.Appointment{}, err
	}

	practitioner, err := uc.PractitionerFhirClient.FindPractitionerByID(ctx, practitionerID)
	if err != nil {
		return responses.Appointment{}, err
	}

	transaction, err := uc.TransactionRepository.FindByID(ctx, fhirAppointment.ID)
	if err != nil {
		return responses.Appointment{}, err
	}

	return responses.Appointment{
		ID:              fhirAppointment.ID,
		Status:          fhirAppointment.Status,
		AppointmentTime: fhirAppointment.Start,
		Description:     fhirAppointment.Description,
		MinutesDuration: fhirAppointment.MinutesDuration,
		PatientID:       patientID,
		PatientName:     utils.GetFullName(patient.Name),
		ClinicianID:     practitionerID,
		ClinicianName:   utils.GetFullName(practitioner.Name),
		PaymentStatus:   string(transaction.StatusPayment),
		PaymentLink:     transaction.PaymentLink,
	}, nil
}

func (uc *appointmentUsecase) FindUpcomingAppointment(ctx context.Context, sessionData string, queryParamsRequest *requests.QueryParams) (responses.Appointment, error) {
	session, err := uc.SessionService.ParseSessionData(ctx, sessionData)
	if err != nil {
		return responses.Appointment{}, err
	}

	if session.IsPatient() {
		queryParamsRequest.PatientID = session.PatientID
	} else if session.IsPractitioner() {
		queryParamsRequest.PractitionerID = session.PractitionerID
	}

	appointments, err := uc.AppointmentFhirClient.FindAll(ctx, queryParamsRequest)
	if err != nil {
		return responses.Appointment{}, err
	}

	if len(appointments) == 0 {
		return responses.Appointment{}, nil
	}

	var response []responses.Appointment
	for _, appt := range appointments {
		var apptResp responses.Appointment
		if session.IsPatient() {
			apptResp, err = uc.buildPatientAppointmentResponse(ctx, appt)
		} else if session.IsPractitioner() {
			apptResp, err = uc.buildPractitionerAppointmentResponse(ctx, appt)
		}
		if err != nil {
			return responses.Appointment{}, err
		}
		response = append(response, apptResp)
	}

	return response[0], nil
}

func (uc *appointmentUsecase) buildPatientAppointmentResponse(ctx context.Context, fhirAppointment fhir_dto.Appointment) (responses.Appointment, error) {
	patientID, err := utils.FindPatientIDFromFhirAppointment(ctx, fhirAppointment)
	if err != nil {
		return responses.Appointment{}, err
	}

	patient, err := uc.PatientFhirClient.FindPatientByID(ctx, patientID)
	if err != nil {
		return responses.Appointment{}, err
	}

	transaction, err := uc.TransactionRepository.FindByID(ctx, fhirAppointment.ID)
	if err != nil {
		return responses.Appointment{}, err
	}

	return responses.Appointment{
		ID:              fhirAppointment.ID,
		Status:          fhirAppointment.Status,
		AppointmentTime: fhirAppointment.Start,
		Description:     fhirAppointment.Description,
		MinutesDuration: fhirAppointment.MinutesDuration,
		PatientID:       patientID,
		PatientName:     utils.GetFullName(patient.Name),
		PaymentStatus:   string(transaction.StatusPayment),
		PaymentLink:     transaction.PaymentLink,
	}, nil
}

func (uc *appointmentUsecase) buildPractitionerAppointmentResponse(ctx context.Context, fhirAppointment fhir_dto.Appointment) (responses.Appointment, error) {
	practitionerID, err := utils.FindPractitionerIDFromFhirAppointment(ctx, fhirAppointment)
	if err != nil {
		return responses.Appointment{}, err
	}

	practitioner, err := uc.PractitionerFhirClient.FindPractitionerByID(ctx, practitionerID)
	if err != nil {
		return responses.Appointment{}, err
	}

	transaction, err := uc.TransactionRepository.FindByID(ctx, fhirAppointment.ID)
	if err != nil {
		return responses.Appointment{}, err
	}

	return responses.Appointment{
		ID:              fhirAppointment.ID,
		Status:          fhirAppointment.Status,
		AppointmentTime: fhirAppointment.Start,
		Description:     fhirAppointment.Description,
		MinutesDuration: fhirAppointment.MinutesDuration,
		ClinicianID:     practitionerID,
		ClinicianName:   utils.GetFullName(practitioner.Name),
		PaymentStatus:   string(transaction.StatusPayment),
		PaymentLink:     transaction.PaymentLink,
	}, nil
}

func (uc *appointmentUsecase) CreateAppointment(ctx context.Context, sessionData string, request *requests.CreateAppointmentRequest) (*responses.CreateAppointment, error) {
	session, err := uc.parseAndValidatePatientSession(ctx, sessionData)
	if err != nil {
		return nil, err
	}
	request.PatientID = session.PatientID

	request.StartTime, err = parseAppointmentStartTime(request.Date, request.Time)
	if err != nil {
		return nil, exceptions.ErrCannotParseTime(err)
	}

	slotsToBook, lastSlot, err := uc.createSlots(ctx, request)
	if err != nil {
		return nil, err
	}

	savedAppointment, err := uc.createFhirAppointment(ctx, request, lastSlot.End, slotsToBook)
	if err != nil {
		return nil, err
	}

	totalPrice := request.NumberOfSessions * request.PricePerSession
	paymentResponse, err := uc.createPayment(ctx, savedAppointment.ID, totalPrice)
	if err != nil {
		return nil, err
	}

	err = uc.saveTransaction(ctx, paymentResponse, request, totalPrice)
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

func (uc *appointmentUsecase) parseAndValidatePatientSession(ctx context.Context, sessionData string) (*models.Session, error) {
	session, err := uc.SessionService.ParseSessionData(ctx, sessionData)
	if err != nil {
		return nil, err
	}
	if session.IsNotPatient() {
		return nil, exceptions.ErrNotMatchRoleType(nil)
	}
	return session, nil
}

func parseAppointmentStartTime(dateStr, timeStr string) (time.Time, error) {
	return time.Parse("2006-01-02 15:04", fmt.Sprintf("%s %s", dateStr, timeStr))
}

func (uc *appointmentUsecase) createSlots(ctx context.Context, request *requests.CreateAppointmentRequest) ([]fhir_dto.Reference, *fhir_dto.Slot, error) {
	var (
		slotsToBook     []fhir_dto.Reference
		lastSlotCreated *fhir_dto.Slot
	)

	for i := 0; i < request.NumberOfSessions; i++ {
		slotStart := request.StartTime.Add(
			time.Duration(i) * time.Duration(uc.InternalConfig.App.SessionMultiplierInMinutes) * time.Minute,
		)
		slotEnd := slotStart.Add(
			time.Duration(uc.InternalConfig.App.SessionMultiplierInMinutes) * time.Minute,
		)

		slotFhirRequest := &fhir_dto.Slot{
			ResourceType: constvars.ResourceSlot,
			Schedule: fhir_dto.Reference{
				Reference: fmt.Sprintf("%s/%s", constvars.ResourceSchedule, request.ScheduleID),
			},
			Status: constvars.FhirSlotStatusBusy,
			Start:  slotStart,
			End:    slotEnd,
		}

		createdSlot, err := uc.SlotFhirClient.CreateSlot(ctx, slotFhirRequest)
		if err != nil {
			return nil, nil, err
		}

		slotsToBook = append(slotsToBook, fhir_dto.Reference{
			Reference: fmt.Sprintf("%s/%s", constvars.ResourceSlot, createdSlot.ID),
		})

		lastSlotCreated = createdSlot
	}

	return slotsToBook, lastSlotCreated, nil
}

func (uc *appointmentUsecase) createFhirAppointment(ctx context.Context, request *requests.CreateAppointmentRequest, end time.Time, slotsToBook []fhir_dto.Reference) (*fhir_dto.Appointment, error) {
	appointmentFhirRequest := &fhir_dto.Appointment{
		ResourceType: constvars.ResourceAppointment,
		Status:       constvars.FhirAppointmentStatusProposed,
		Start:        request.StartTime,
		End:          end,
		Slot:         slotsToBook,
		Description:  request.ProblemBrief,
		Participant: []fhir_dto.AppointmentParticipant{
			{
				Actor: fhir_dto.Reference{
					Reference: fmt.Sprintf("%s/%s", constvars.ResourcePatient, request.PatientID),
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

	return uc.AppointmentFhirClient.CreateAppointment(ctx, appointmentFhirRequest)
}

func (uc *appointmentUsecase) createPayment(ctx context.Context, partnerTransactionID string, totalPrice int) (*responses.PaymentResponse, error) {
	paymentRoutingRequest := &requests.PaymentRequest{
		UseLinkedAccount:     false,
		PartnerTransactionID: partnerTransactionID,
		NeedFrontend:         true,
		SenderEmail:          uc.InternalConfig.Konsulin.FinanceEmail,
		VADisplayName:        uc.InternalConfig.Konsulin.PaymentDisplayName,
		PaymentExpirationTime: utils.AddAndGetTime(
			constvars.TIME_DIFFERENCE_JAKARTA,
			uc.InternalConfig.App.PaymentExpiredTimeInMinutes,
			10,
		),
		ReceiveAmount: totalPrice,
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
				RecipientBank:    uc.InternalConfig.Konsulin.BankCode,
				RecipientAccount: uc.InternalConfig.Konsulin.BankAccountNumber,
				RecipientEmail:   uc.InternalConfig.Konsulin.FinanceEmail,
				RecipientAmount:  totalPrice,
			},
		},
	}

	paymentRequestDTO := utils.MapPaymentRequestToDTO(paymentRoutingRequest)
	return uc.OyService.CreatePaymentRouting(ctx, paymentRequestDTO)
}

func (uc *appointmentUsecase) saveTransaction(ctx context.Context, paymentResponse *responses.PaymentResponse, request *requests.CreateAppointmentRequest, totalPrice int) error {
	transaction := &models.Transaction{
		ID:                      paymentResponse.PartnerTrxID,
		PatientID:               request.PatientID,
		PractitionerID:          request.ClinicianID,
		LengthMinutesPerSession: uc.InternalConfig.App.SessionMultiplierInMinutes,
		SessionTotal:            request.NumberOfSessions,
		Currency:                constvars.CurrencyIndonesianRupiah,
		PaymentLink:             paymentResponse.PaymentInfo.PaymentCheckoutURL,
		Amount:                  float64(totalPrice),
		StatusPayment:           models.Pending,
		RefundStatus:            models.None,
		SessionType:             models.TransactionSessionType(request.SessionType),
	}

	_, err := uc.TransactionRepository.CreateTransaction(ctx, transaction)
	return err
}
