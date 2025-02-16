package patients

import (
	"context"
	"fmt"
	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/contracts"
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

type patientUsecase struct {
	PractitionerFhirClient     contracts.PractitionerFhirClient
	PractitionerRoleFhirClient contracts.PractitionerRoleFhirClient
	ScheduleFhirClient         contracts.ScheduleFhirClient
	SlotFhirClient             contracts.SlotFhirClient
	AppointmentFhirClient      contracts.AppointmentFhirClient
	SessionService             contracts.SessionService
	OyService                  contracts.PaymentGatewayService
	InternalConfig             *config.InternalConfig
	Log                        *zap.Logger
}

var (
	patientUsecaseInstance contracts.PatientUsecase
	oncePatientUsecase     sync.Once
)

func NewPatientUsecase(
	practitionerFhirClient contracts.PractitionerFhirClient,
	practitionerRoleFhirClient contracts.PractitionerRoleFhirClient,
	scheduleFhirClient contracts.ScheduleFhirClient,
	slotFhirClient contracts.SlotFhirClient,
	appointmentFhirClient contracts.AppointmentFhirClient,
	sessionService contracts.SessionService,
	oyService contracts.PaymentGatewayService,
	internalConfig *config.InternalConfig,
	logger *zap.Logger,
) contracts.PatientUsecase {
	oncePatientUsecase.Do(func() {
		instance := &patientUsecase{
			PractitionerFhirClient:     practitionerFhirClient,
			PractitionerRoleFhirClient: practitionerRoleFhirClient,
			ScheduleFhirClient:         scheduleFhirClient,
			SlotFhirClient:             slotFhirClient,
			AppointmentFhirClient:      appointmentFhirClient,
			SessionService:             sessionService,
			OyService:                  oyService,
			InternalConfig:             internalConfig,
			Log:                        logger,
		}
		patientUsecaseInstance = instance
	})
	return patientUsecaseInstance
}
func (uc *patientUsecase) CreateAppointment(ctx context.Context, sessionData string, request *requests.CreateAppointmentRequest) (*responses.CreateAppointment, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	uc.Log.Info("patientUsecase.CreateAppointment called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	session, err := uc.SessionService.ParseSessionData(ctx, sessionData)
	if err != nil {
		uc.Log.Error("patientUsecase.CreateAppointment error parsing session data",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, err
	}

	if session.IsNotPatient() {
		uc.Log.Error("patientUsecase.CreateAppointment error: session role is not patient",
			zap.String(constvars.LoggingRequestIDKey, requestID),
		)
		return nil, exceptions.ErrNotMatchRoleType(nil)
	}

	appointmentStartTime, err := time.Parse("2006-01-02 15:04", fmt.Sprintf("%s %s", request.Date, request.Time))
	if err != nil {
		uc.Log.Error("patientUsecase.CreateAppointment error parsing appointment start time",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCannotParseTime(err)
	}
	uc.Log.Info("patientUsecase.CreateAppointment appointment start time parsed",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Time("appointment_start", appointmentStartTime),
	)

	var slotsToBook []fhir_dto.Reference
	var lastSlotBooked *fhir_dto.Slot
	for i := 0; i < request.NumberOfSessions; i++ {
		startTime := appointmentStartTime.Add(time.Duration(i) * 30 * time.Minute)
		endTime := startTime.Add(30 * time.Minute)

		uc.Log.Info("patientUsecase.CreateAppointment creating slot",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Int("session_index", i),
			zap.Time(constvars.LoggingSlotsStartKey, startTime),
			zap.Time(constvars.LoggingSlotsEndKey, endTime),
		)

		slotFhirRequest := &fhir_dto.Slot{
			ResourceType: constvars.ResourceSlot,
			Schedule: fhir_dto.Reference{
				Reference: fmt.Sprintf("Schedule/%s", request.ScheduleID),
			},
			Status: constvars.FhirSlotStatusBusy,
			Start:  startTime,
			End:    endTime,
		}

		slot, err := uc.SlotFhirClient.CreateSlot(ctx, slotFhirRequest)
		if err != nil {
			uc.Log.Error("patientUsecase.CreateAppointment error creating slot",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, err
		}

		uc.Log.Info("patientUsecase.CreateAppointment slot created",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingSlotsIDKey, slot.ID),
		)

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
	uc.Log.Info("patientUsecase.CreateAppointment built appointment FHIR request",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Time(constvars.LoggingAppointmentStartTimeKey, appointmentFhirRequest.Start),
		zap.Time(constvars.LoggingAppointmentEndTimeKey, appointmentFhirRequest.End),
	)

	savedAppointment, err := uc.AppointmentFhirClient.CreateAppointment(ctx, appointmentFhirRequest)
	if err != nil {
		uc.Log.Error("patientUsecase.CreateAppointment error creating appointment",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, err
	}
	uc.Log.Info("patientUsecase.CreateAppointment appointment created",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingAppointmentIDKey, savedAppointment.ID),
	)

	paymentRoutingRequest := &requests.PaymentRequest{
		UseLinkedAccount:     false,
		PartnerTransactionID: savedAppointment.ID,
		NeedFrontend:         true,
		SenderEmail:          uc.InternalConfig.Konsulin.FinanceEmail,
		VADisplayName:        uc.InternalConfig.Konsulin.PaymentDisplayName,
		ReceiveAmount:        30000,
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
				RecipientAmount:  30000,
			},
		},
	}
	uc.Log.Info("patientUsecase.CreateAppointment built payment routing request",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingAppointmentIDKey, savedAppointment.ID),
	)

	paymentRequestDTO := utils.MapPaymentRequestToDTO(paymentRoutingRequest)

	paymentResponse, err := uc.OyService.CreatePaymentRouting(ctx, paymentRequestDTO)
	if err != nil {
		uc.Log.Error("patientUsecase.CreateAppointment error creating payment routing",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, err
	}
	uc.Log.Info("patientUsecase.CreateAppointment payment routing created",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	uc.Log.Info("patientUsecase.CreateAppointment succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)
	return &responses.CreateAppointment{
		Status: paymentResponse.Status.Message,
	}, nil
}
