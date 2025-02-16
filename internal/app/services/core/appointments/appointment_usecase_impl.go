package appointments

import (
	"context"
	"errors"
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
	"sync"
	"time"

	"go.uber.org/zap"
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
	LockService            contracts.LockerService
	InternalConfig         *config.InternalConfig
	Log                    *zap.Logger
}

var (
	appointmentUsecaseInstance contracts.AppointmentUsecase
	onceAppointmentUsecase     sync.Once
)

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
	lockService contracts.LockerService,
	internalConfig *config.InternalConfig,
	logger *zap.Logger,
) contracts.AppointmentUsecase {
	onceAppointmentUsecase.Do(func() {
		instance := &appointmentUsecase{
			TransactionRepository:  transactionRepository,
			ClinicianService:       clinicianService,
			AppointmentFhirClient:  appointmentFhirClient,
			PatientFhirClient:      patientFhirClient,
			PractitionerFhirClient: practitionerFhirClient,
			SlotFhirClient:         slotFhirClient,
			RedisRepository:        redisRepository,
			SessionService:         sessionService,
			OyService:              oyService,
			LockService:            lockService,
			InternalConfig:         internalConfig,
			Log:                    logger,
		}
		appointmentUsecaseInstance = instance
	})
	return appointmentUsecaseInstance
}

func (uc *appointmentUsecase) FindAll(ctx context.Context, sessionData string, queryParamsRequest *requests.QueryParams) ([]responses.Appointment, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	uc.Log.Info("appointmentUsecase.FindAll called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	session, err := uc.SessionService.ParseSessionData(ctx, sessionData)
	if err != nil {
		uc.Log.Error("appointmentUsecase.FindAll error parsing session data",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, err
	}

	uc.Log.Info("appointmentUsecase.FindAll session parsed",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Any(constvars.LoggingSessionDataKey, session),
	)

	if session.IsPatient() {
		queryParamsRequest.PatientID = session.PatientID
	} else if session.IsPractitioner() {
		queryParamsRequest.PractitionerID = session.PractitionerID
	}

	appointments, err := uc.AppointmentFhirClient.FindAll(ctx, queryParamsRequest)
	if err != nil {
		uc.Log.Error("appointmentUsecase.FindAll error fetching appointments from FHIR client",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, err
	}

	uc.Log.Info("appointmentUsecase.FindAll appointments retrieved",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Int(constvars.LoggingAppointmentCountKey, len(appointments)),
	)

	response := make([]responses.Appointment, 0, len(appointments))
	for _, eachAppointment := range appointments {
		appointmentResp, err := uc.buildAppointmentResponse(ctx, eachAppointment)
		if err != nil {
			uc.Log.Error("appointmentUsecase.FindAll error building appointment response",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, err
		}
		response = append(response, appointmentResp)
	}

	uc.Log.Info("appointmentUsecase.FindAll succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Int(constvars.LoggingResponseCountKey, len(response)),
	)
	return response, nil
}

func (uc *appointmentUsecase) buildAppointmentResponse(ctx context.Context, fhirAppointment fhir_dto.Appointment) (responses.Appointment, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	uc.Log.Info("appointmentUsecase.buildAppointmentResponse called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingAppointmentIDKey, fhirAppointment.ID),
	)

	patientID, err := utils.FindPatientIDFromFhirAppointment(ctx, fhirAppointment)
	if err != nil {
		uc.Log.Error("appointmentUsecase.buildAppointmentResponse error finding patientID",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return responses.Appointment{}, err
	}

	practitionerID, err := utils.FindPractitionerIDFromFhirAppointment(ctx, fhirAppointment)
	if err != nil {
		uc.Log.Error("appointmentUsecase.buildAppointmentResponse error finding practitionerID",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return responses.Appointment{}, err
	}

	patient, err := uc.PatientFhirClient.FindPatientByID(ctx, patientID)
	if err != nil {
		uc.Log.Error("appointmentUsecase.buildAppointmentResponse error fetching patient",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingPatientIDKey, patientID),
			zap.Error(err),
		)
		return responses.Appointment{}, err
	}

	practitioner, err := uc.PractitionerFhirClient.FindPractitionerByID(ctx, practitionerID)
	if err != nil {
		uc.Log.Error("appointmentUsecase.buildAppointmentResponse error fetching practitioner",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingPractitionerIDKey, practitionerID),
			zap.Error(err),
		)
		return responses.Appointment{}, err
	}

	transaction, err := uc.TransactionRepository.FindByID(ctx, fhirAppointment.ID)
	if err != nil {
		uc.Log.Error("appointmentUsecase.buildAppointmentResponse error fetching transaction",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingAppointmentIDKey, fhirAppointment.ID),
			zap.Error(err),
		)
		return responses.Appointment{}, err
	}

	appointment := responses.Appointment{
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
	}

	uc.Log.Info("appointmentUsecase.buildAppointmentResponse succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)
	return appointment, nil
}

func (uc *appointmentUsecase) FindUpcomingAppointment(ctx context.Context, sessionData string, queryParamsRequest *requests.QueryParams) (responses.Appointment, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	uc.Log.Info("appointmentUsecase.FindUpcomingAppointment called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	session, err := uc.SessionService.ParseSessionData(ctx, sessionData)
	if err != nil {
		uc.Log.Error("appointmentUsecase.FindUpcomingAppointment error parsing session data",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return responses.Appointment{}, err
	}

	if session.IsPatient() {
		queryParamsRequest.PatientID = session.PatientID
	} else if session.IsPractitioner() {
		queryParamsRequest.PractitionerID = session.PractitionerID
	}

	appointments, err := uc.AppointmentFhirClient.FindAll(ctx, queryParamsRequest)
	if err != nil {
		uc.Log.Error("appointmentUsecase.FindUpcomingAppointment error fetching appointments",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return responses.Appointment{}, err
	}

	if len(appointments) == 0 {
		uc.Log.Info("appointmentUsecase.FindUpcomingAppointment no upcoming appointments found",
			zap.String(constvars.LoggingRequestIDKey, requestID),
		)
		return responses.Appointment{}, nil
	}

	uc.Log.Info("appointmentUsecase.FindUpcomingAppointment appointments retrieved",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Int(constvars.LoggingAppointmentCountKey, len(appointments)),
	)

	var response []responses.Appointment
	for _, appt := range appointments {
		var apptResp responses.Appointment
		if session.IsPatient() {
			apptResp, err = uc.buildPatientAppointmentResponse(ctx, appt)
		} else if session.IsPractitioner() {
			apptResp, err = uc.buildPractitionerAppointmentResponse(ctx, appt)
		}
		if err != nil {
			uc.Log.Error("appointmentUsecase.FindUpcomingAppointment error building appointment response",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return responses.Appointment{}, err
		}
		response = append(response, apptResp)
	}

	uc.Log.Info("appointmentUsecase.FindUpcomingAppointment succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Any("upcoming_appointment", response[0]),
	)
	return response[0], nil
}

func (uc *appointmentUsecase) buildPatientAppointmentResponse(ctx context.Context, fhirAppointment fhir_dto.Appointment) (responses.Appointment, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	uc.Log.Info("appointmentUsecase.buildPatientAppointmentResponse called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingAppointmentIDKey, fhirAppointment.ID),
	)

	patientID, err := utils.FindPatientIDFromFhirAppointment(ctx, fhirAppointment)
	if err != nil {
		uc.Log.Error("appointmentUsecase.buildPatientAppointmentResponse error finding patientID",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return responses.Appointment{}, err
	}

	patient, err := uc.PatientFhirClient.FindPatientByID(ctx, patientID)
	if err != nil {
		uc.Log.Error("appointmentUsecase.buildPatientAppointmentResponse error fetching patient",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingPatientIDKey, patientID),
			zap.Error(err),
		)
		return responses.Appointment{}, err
	}

	transaction, err := uc.TransactionRepository.FindByID(ctx, fhirAppointment.ID)
	if err != nil {
		uc.Log.Error("appointmentUsecase.buildPatientAppointmentResponse error fetching transaction",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingAppointmentIDKey, fhirAppointment.ID),
			zap.Error(err),
		)
		return responses.Appointment{}, err
	}

	appointment := responses.Appointment{
		ID:              fhirAppointment.ID,
		Status:          fhirAppointment.Status,
		AppointmentTime: fhirAppointment.Start,
		Description:     fhirAppointment.Description,
		MinutesDuration: fhirAppointment.MinutesDuration,
		PatientID:       patientID,
		PatientName:     utils.GetFullName(patient.Name),
		PaymentStatus:   string(transaction.StatusPayment),
		PaymentLink:     transaction.PaymentLink,
	}

	uc.Log.Info("appointmentUsecase.buildPatientAppointmentResponse succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)
	return appointment, nil
}

func (uc *appointmentUsecase) buildPractitionerAppointmentResponse(ctx context.Context, fhirAppointment fhir_dto.Appointment) (responses.Appointment, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	uc.Log.Info("appointmentUsecase.buildPractitionerAppointmentResponse called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingAppointmentIDKey, fhirAppointment.ID),
	)

	practitionerID, err := utils.FindPractitionerIDFromFhirAppointment(ctx, fhirAppointment)
	if err != nil {
		uc.Log.Error("appointmentUsecase.buildPractitionerAppointmentResponse error finding practitionerID",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return responses.Appointment{}, err
	}

	practitioner, err := uc.PractitionerFhirClient.FindPractitionerByID(ctx, practitionerID)
	if err != nil {
		uc.Log.Error("appointmentUsecase.buildPractitionerAppointmentResponse error fetching practitioner",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingPractitionerIDKey, practitionerID),
			zap.Error(err),
		)
		return responses.Appointment{}, err
	}

	transaction, err := uc.TransactionRepository.FindByID(ctx, fhirAppointment.ID)
	if err != nil {
		uc.Log.Error("appointmentUsecase.buildPractitionerAppointmentResponse error fetching transaction",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingAppointmentIDKey, fhirAppointment.ID),
			zap.Error(err),
		)
		return responses.Appointment{}, err
	}

	appointment := responses.Appointment{
		ID:              fhirAppointment.ID,
		Status:          fhirAppointment.Status,
		AppointmentTime: fhirAppointment.Start,
		Description:     fhirAppointment.Description,
		MinutesDuration: fhirAppointment.MinutesDuration,
		ClinicianID:     practitionerID,
		ClinicianName:   utils.GetFullName(practitioner.Name),
		PaymentStatus:   string(transaction.StatusPayment),
		PaymentLink:     transaction.PaymentLink,
	}

	uc.Log.Info("appointmentUsecase.buildPractitionerAppointmentResponse succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)
	return appointment, nil
}

func (uc *appointmentUsecase) CreateAppointment(ctx context.Context, sessionData string, request *requests.CreateAppointmentRequest) (*responses.CreateAppointment, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	uc.Log.Info("appointmentUsecase.CreateAppointment called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Any("request", request),
	)

	session, err := uc.parseAndValidatePatientSession(ctx, sessionData)
	if err != nil {
		uc.Log.Error("appointmentUsecase.CreateAppointment error validating patient session",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, err
	}
	request.PatientID = session.PatientID

	request.StartTime, err = parseAppointmentStartTime(request.Date, request.Time)
	if err != nil {
		uc.Log.Error("appointmentUsecase.CreateAppointment error parsing appointment start time",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCannotParseTime(err)
	}
	uc.Log.Info("appointmentUsecase.CreateAppointment start time parsed",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	isAvailable, err := uc.checkSlotAvailability(ctx, request)
	if err != nil {
		uc.Log.Error("appointmentUsecase.CreateAppointment error checking slot availability",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, err
	}
	if !isAvailable {
		customErrMessage := errors.New("requested time is already booked or not available")
		uc.Log.Warn("appointmentUsecase.CreateAppointment slot not available",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(customErrMessage),
		)
		return nil, exceptions.ErrClientCustomMessage(customErrMessage)
	}

	slotsToBook, lastSlot, err := uc.createSlots(ctx, request)
	if err != nil {
		uc.Log.Error("appointmentUsecase.CreateAppointment error creating slots",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, err
	}
	uc.Log.Info("appointmentUsecase.CreateAppointment slots created",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Int(constvars.LoggingSlotsCountKey, len(slotsToBook)),
	)

	savedAppointment, err := uc.createFhirAppointment(ctx, request, lastSlot.End, slotsToBook)
	if err != nil {
		uc.Log.Error("appointmentUsecase.CreateAppointment error creating FHIR appointment",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, err
	}
	uc.Log.Info("appointmentUsecase.CreateAppointment FHIR appointment created",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingAppointmentIDKey, savedAppointment.ID),
	)

	totalPrice := request.NumberOfSessions * request.PricePerSession

	// Uncomment to activeate the createPayment again.
	/*
		paymentResponse, err := uc.createPayment(ctx, savedAppointment.ID, totalPrice)
		if err != nil {
			uc.Log.Error("appointmentUsecase.CreateAppointment error creating payment",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, err
		}
		uc.Log.Info("appointmentUsecase.CreateAppointment payment created",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Any("payment_response", paymentResponse),
		)
	*/

	err = uc.saveTransaction(ctx, savedAppointment, request, totalPrice)
	if err != nil {
		uc.Log.Error("appointmentUsecase.CreateAppointment error saving transaction",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, err
	}
	uc.Log.Info("appointmentUsecase.CreateAppointment transaction saved",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	response := &responses.CreateAppointment{
		ID:              savedAppointment.ID,
		Status:          savedAppointment.Status,
		ClinicianID:     request.ClinicianID,
		PatientID:       request.PatientID,
		AppointmentTime: savedAppointment.Start,
		Description:     request.ProblemBrief,
		MinutesDuration: request.NumberOfSessions * uc.InternalConfig.App.SessionMultiplierInMinutes,
	}

	uc.Log.Info("appointmentUsecase.CreateAppointment succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)
	return response, nil
}

func (uc *appointmentUsecase) parseAndValidatePatientSession(ctx context.Context, sessionData string) (*models.Session, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	uc.Log.Info("appointmentUsecase.parseAndValidatePatientSession called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)
	session, err := uc.SessionService.ParseSessionData(ctx, sessionData)
	if err != nil {
		uc.Log.Error("appointmentUsecase.parseAndValidatePatientSession error parsing session data",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, err
	}
	if session.IsNotPatient() {
		uc.Log.Error("appointmentUsecase.parseAndValidatePatientSession role mismatch: not a patient",
			zap.String(constvars.LoggingRequestIDKey, requestID),
		)
		return nil, exceptions.ErrNotMatchRoleType(nil)
	}
	uc.Log.Info("appointmentUsecase.parseAndValidatePatientSession succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Any(constvars.LoggingSessionDataKey, session),
	)
	return session, nil
}

func parseAppointmentStartTime(dateStr, timeStr string) (time.Time, error) {
	return time.Parse("2006-01-02 15:04", fmt.Sprintf("%s %s", dateStr, timeStr))
}

func (uc *appointmentUsecase) checkSlotAvailability(ctx context.Context, req *requests.CreateAppointmentRequest) (bool, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	uc.Log.Info("appointmentUsecase.checkSlotAvailability called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	var timeRanges []struct {
		Start time.Time
		End   time.Time
	}

	for i := 0; i < req.NumberOfSessions; i++ {
		slotStart := req.StartTime.Add(time.Duration(i) * time.Duration(uc.InternalConfig.App.SessionMultiplierInMinutes) * time.Minute)
		slotEnd := slotStart.Add(time.Duration(uc.InternalConfig.App.SessionMultiplierInMinutes) * time.Minute)
		timeRanges = append(timeRanges, struct {
			Start time.Time
			End   time.Time
		}{
			Start: slotStart,
			End:   slotEnd,
		})
	}

	for _, timeRange := range timeRanges {
		existingBusySlots, err := uc.SlotFhirClient.FindSlotByScheduleAndTimeRange(ctx, req.ScheduleID, timeRange.Start, timeRange.End)
		if err != nil {
			uc.Log.Error("appointmentUsecase.checkSlotAvailability error fetching busy slots",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Time(constvars.LoggingSlotsStartKey, timeRange.Start),
				zap.Time(constvars.LoggingSlotsEndKey, timeRange.End),
				zap.Error(err),
			)
			return false, err
		}
		if len(existingBusySlots) > 0 {
			uc.Log.Warn("appointmentUsecase.checkSlotAvailability found busy slot",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Time(constvars.LoggingSlotsStartKey, timeRange.Start),
				zap.Time(constvars.LoggingSlotsEndKey, timeRange.End),
			)
			return false, nil
		}
	}

	uc.Log.Info("appointmentUsecase.checkSlotAvailability available",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)
	return true, nil
}

func (uc *appointmentUsecase) createSlots(ctx context.Context, request *requests.CreateAppointmentRequest) ([]fhir_dto.Reference, *fhir_dto.Slot, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	uc.Log.Info("appointmentUsecase.createSlots called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	var (
		slotsToBook     []fhir_dto.Reference
		lastSlotCreated *fhir_dto.Slot
	)

	for i := 0; i < request.NumberOfSessions; i++ {
		slotStart := request.StartTime.Add(time.Duration(i) * time.Duration(uc.InternalConfig.App.SessionMultiplierInMinutes) * time.Minute)
		slotEnd := slotStart.Add(time.Duration(uc.InternalConfig.App.SessionMultiplierInMinutes) * time.Minute)

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
			uc.Log.Error("appointmentUsecase.createSlots error creating slot",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, nil, err
		}

		uc.Log.Info("appointmentUsecase.createSlots slot created",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingSlotsIDKey, createdSlot.ID),
			zap.Time(constvars.LoggingSlotsStartKey, createdSlot.Start),
			zap.Time(constvars.LoggingSlotsEndKey, createdSlot.End),
		)

		slotsToBook = append(slotsToBook, fhir_dto.Reference{
			Reference: fmt.Sprintf("%s/%s", constvars.ResourceSlot, createdSlot.ID),
		})
		lastSlotCreated = createdSlot
	}

	uc.Log.Info("appointmentUsecase.createSlots succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Int(constvars.LoggingSlotsCountKey, len(slotsToBook)),
	)
	return slotsToBook, lastSlotCreated, nil
}

func (uc *appointmentUsecase) createFhirAppointment(ctx context.Context, request *requests.CreateAppointmentRequest, end time.Time, slotsToBook []fhir_dto.Reference) (*fhir_dto.Appointment, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	uc.Log.Info("appointmentUsecase.createFhirAppointment called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

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

	createdAppointment, err := uc.AppointmentFhirClient.CreateAppointment(ctx, appointmentFhirRequest)
	if err != nil {
		uc.Log.Error("appointmentUsecase.createFhirAppointment error creating appointment",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, err
	}

	uc.Log.Info("appointmentUsecase.createFhirAppointment succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingAppointmentIDKey, createdAppointment.ID),
	)
	return createdAppointment, nil
}

func (uc *appointmentUsecase) createPayment(ctx context.Context, partnerTransactionID string, totalPrice int) (*responses.PaymentResponse, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	uc.Log.Info("appointmentUsecase.createPayment called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

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
	paymentResponse, err := uc.OyService.CreatePaymentRouting(ctx, paymentRequestDTO)
	if err != nil {
		uc.Log.Error("appointmentUsecase.createPayment error creating payment routing",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, err
	}

	uc.Log.Info("appointmentUsecase.createPayment succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)
	return paymentResponse, nil
}

func (uc *appointmentUsecase) saveTransaction(ctx context.Context, savedAppointment *fhir_dto.Appointment, request *requests.CreateAppointmentRequest, totalPrice int) error {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	uc.Log.Info("appointmentUsecase.saveTransaction called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingAppointmentIDKey, savedAppointment.ID),
	)

	transaction := &models.Transaction{
		ID:                      savedAppointment.ID,
		PatientID:               request.PatientID,
		PractitionerID:          request.ClinicianID,
		LengthMinutesPerSession: uc.InternalConfig.App.SessionMultiplierInMinutes,
		SessionTotal:            request.NumberOfSessions,
		Currency:                constvars.CurrencyIndonesianRupiah,
		Amount:                  float64(totalPrice),
		SessionType:             models.TransactionSessionType(request.SessionType),
	}

	_, err := uc.TransactionRepository.CreateTransaction(ctx, transaction)
	if err != nil {
		uc.Log.Error("appointmentUsecase.saveTransaction error creating transaction",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return err
	}

	uc.Log.Info("appointmentUsecase.saveTransaction succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)
	return nil
}
