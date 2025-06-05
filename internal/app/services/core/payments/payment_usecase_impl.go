package payments

import (
	"context"
	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/requests"
	"sync"

	"go.uber.org/zap"
)

type paymentUsecase struct {
	TransactionRepository contracts.TransactionRepository
	InternalConfig        *config.InternalConfig
	Log                   *zap.Logger
}

var (
	paymentUsecaseInstance contracts.PaymentUsecase
	oncePaymentUsecase     sync.Once
)

func NewPaymentUsecase(
	transactionRepository contracts.TransactionRepository,
	internalConfig *config.InternalConfig,
	logger *zap.Logger,
) contracts.PaymentUsecase {
	oncePaymentUsecase.Do(func() {
		instance := &paymentUsecase{
			TransactionRepository: transactionRepository,
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
