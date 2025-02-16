package payments

import (
	"context"
	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/app/models"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/fhir_dto"
	"sync"

	"go.uber.org/zap"
)

type paymentUsecase struct {
	TransactionRepository contracts.TransactionRepository
	AppointmentFhirClient contracts.AppointmentFhirClient
	InternalConfig        *config.InternalConfig
	Log                   *zap.Logger
}

var (
	paymentUsecaseInstance contracts.PaymentUsecase
	oncePaymentUsecase     sync.Once
)

func NewPaymentUsecase(
	transactionRepository contracts.TransactionRepository,
	appointmentFhirClient contracts.AppointmentFhirClient,
	internalConfig *config.InternalConfig,
	logger *zap.Logger,
) contracts.PaymentUsecase {
	oncePaymentUsecase.Do(func() {
		instance := &paymentUsecase{
			TransactionRepository: transactionRepository,
			AppointmentFhirClient: appointmentFhirClient,
			InternalConfig:        internalConfig,
			Log:                   logger,
		}
		paymentUsecaseInstance = instance
	})
	return paymentUsecaseInstance
}
func (uc *paymentUsecase) PaymentRoutingCallback(ctx context.Context, request *requests.PaymentRoutingCallback) error {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	uc.Log.Info("paymentUsecase.PaymentRoutingCallback called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Any(constvars.LoggingRequestKey, request),
	)

	appointment, err := uc.AppointmentFhirClient.FindAppointmentByID(ctx, request.PartnerTrxID)
	if err != nil {
		uc.Log.Error("paymentUsecase.PaymentRoutingCallback error fetching appointment",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return err
	}
	uc.Log.Info("paymentUsecase.PaymentRoutingCallback fetched appointment",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingAppointmentIDKey, appointment.ID),
	)

	transaction, err := uc.TransactionRepository.FindByID(ctx, request.PartnerTrxID)
	if err != nil {
		uc.Log.Error("paymentUsecase.PaymentRoutingCallback error fetching transaction",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return err
	}
	uc.Log.Info("paymentUsecase.PaymentRoutingCallback fetched transaction",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingTransactionIDKey, transaction.ID),
	)

	if request.PaymentStatus == constvars.OY_COMPLETE_STATUS {
		uc.Log.Info("paymentUsecase.PaymentRoutingCallback processing complete payment",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingOyPaymentStatusKey, request.PaymentStatus),
		)
		appointment.Status = constvars.FhirAppointmentStatusBooked
		transaction.StatusPayment = models.Completed
	} else if request.PaymentStatus == constvars.OY_EXPIRED_STATUS || request.PaymentStatus == constvars.OY_INCOMPLETE_STATUS || request.PaymentStatus == constvars.OY_PAYMENT_FAILED_STATUS {
		uc.Log.Info("paymentUsecase.PaymentRoutingCallback processing non-complete payment",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingOyPaymentStatusKey, request.PaymentStatus),
		)
		appointment.Status = constvars.FhirAppointmentStatusCancelled
		transaction.StatusPayment = models.Failed
		appointment.ReasonCode = []fhir_dto.CodeableConcept{
			{
				Coding: []fhir_dto.Coding{
					{
						Code:    "financial",
						Display: "Cancelled due to lack of payment",
					},
				},
				Text: "Cancelled because the payment was not made.",
			},
		}
	} else {
		uc.Log.Warn("paymentUsecase.PaymentRoutingCallback encountered unhandled payment status",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingOyPaymentStatusKey, request.PaymentStatus),
		)
	}

	updatedAppointment, err := uc.AppointmentFhirClient.UpdateAppointment(ctx, appointment)
	if err != nil {
		uc.Log.Error("paymentUsecase.PaymentRoutingCallback error updating appointment",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return err
	}
	uc.Log.Info("paymentUsecase.PaymentRoutingCallback updated appointment",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingAppointmentIDKey, updatedAppointment.ID),
	)

	updatedTransaction, err := uc.TransactionRepository.UpdateTransaction(ctx, transaction)
	if err != nil {
		uc.Log.Error("paymentUsecase.PaymentRoutingCallback error updating transaction",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return err
	}
	uc.Log.Info("paymentUsecase.PaymentRoutingCallback updated transaction",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingTransactionIDKey, updatedTransaction.ID),
	)

	uc.Log.Info("paymentUsecase.PaymentRoutingCallback completed successfully",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)
	return nil
}
