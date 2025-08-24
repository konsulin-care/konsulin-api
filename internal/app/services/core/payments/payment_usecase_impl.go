package payments

import (
	"context"
	"encoding/json"
	"fmt"
	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/app/services/shared/storage"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/dto/responses"
	"konsulin-service/internal/pkg/exceptions"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

type paymentUsecase struct {
	TransactionRepository contracts.TransactionRepository
	InternalConfig        *config.InternalConfig
	Log                   *zap.Logger
	PatientFhirClient     contracts.PatientFhirClient
	Storage               *storage.ServiceRequestStorage
	PaymentGateway        contracts.PaymentGatewayService
}

var (
	paymentUsecaseInstance contracts.PaymentUsecase
	oncePaymentUsecase     sync.Once
)

func NewPaymentUsecase(
	transactionRepository contracts.TransactionRepository,
	internalConfig *config.InternalConfig,
	patientFhirClient contracts.PatientFhirClient,
	storageService *storage.ServiceRequestStorage,
	paymentGateway contracts.PaymentGatewayService,
	logger *zap.Logger,
) contracts.PaymentUsecase {
	oncePaymentUsecase.Do(func() {
		instance := &paymentUsecase{
			TransactionRepository: transactionRepository,
			InternalConfig:        internalConfig,
			Log:                   logger,
			PatientFhirClient:     patientFhirClient,
			Storage:               storageService,
			PaymentGateway:        paymentGateway,
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

func (uc *paymentUsecase) CreatePay(ctx context.Context, req *requests.CreatePayRequest) (*responses.CreatePayResponse, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	uc.Log.Info("paymentUsecase.CreatePay called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	// 1) Reject guest role
	roles, _ := ctx.Value("roles").([]string)
	if len(roles) == 0 || (len(roles) == 1 && strings.EqualFold(roles[0], constvars.KonsulinRoleGuest)) {
		return nil, exceptions.ErrAuthInvalidRole(fmt.Errorf("guest not allowed"))
	}

	// 2) Extract uid from context
	uid, _ := ctx.Value("uid").(string)

	// 3) Extract email from req.Body
	var raw map[string]interface{}
	if err := json.Unmarshal(req.Body, &raw); err != nil {
		return nil, exceptions.ErrCannotParseJSON(err)
	}
	email, _ := raw["email"].(string)
	if strings.TrimSpace(email) == "" {
		return nil, exceptions.ErrClientCustomMessage(fmt.Errorf("email is required in body"))
	}

	// 4) Lookup Patient by email; if none, 404
	patients, err := uc.PatientFhirClient.FindPatientByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	if len(patients) == 0 {
		return nil, exceptions.ErrUserNotExist(fmt.Errorf("no patient found"))
	}
	patientID := patients[0].ID

	// 5) Build instantiateUri and store in ServiceRequest note
	instantiateURI := fmt.Sprintf("%s/hook/%s", strings.TrimRight(uc.InternalConfig.App.BaseUrl, "/"), req.Service)
	occurrence := time.Now().Format("2006-01-02T15:04:05-07:00")
	storageOutput, err := uc.Storage.Create(ctx, &requests.CreateServiceRequestStorageInput{
		UID:            uid,
		PatientID:      patientID,
		InstantiateURI: instantiateURI,
		RawBody:        req.Body,
		Occurrence:     occurrence,
	})
	if err != nil {
		return nil, err
	}
	partnerTrxID := storageOutput.PartnerTrxID

	// 6) Compute amount using safe switch on role
	basePrice, err := uc.getBasePriceFromRoles(roles)
	if err != nil {
		return nil, exceptions.ErrClientCustomMessage(err)
	}
	amount := req.TotalItem * basePrice

	// 7) Prepare OY payment request
	oyReq := &requests.PaymentRequestDTO{
		PartnerUserID:           uid,
		UseLinkedAccount:        false,
		PartnerTransactionID:    partnerTrxID,
		NeedFrontend:            true,
		SenderEmail:             email,
		PaymentExpirationTime:   fmt.Sprintf("%dm", uc.InternalConfig.App.PaymentExpiredTimeInMinutes),
		ReceiveAmount:           amount,
		ListEnablePaymentMethod: uc.InternalConfig.PaymentGateway.ListEnablePaymentMethod,
		ListEnableSOF:           uc.InternalConfig.PaymentGateway.ListEnableSOF,
		VADisplayName:           uc.InternalConfig.Konsulin.PaymentDisplayName,
		PaymentRouting: []requests.PaymentRouting{
			{
				RecipientBank:    uc.InternalConfig.Konsulin.BankCode,
				RecipientAccount: uc.InternalConfig.Konsulin.BankAccountNumber,
				RecipientAmount:  amount,
				RecipientEmail:   uc.InternalConfig.Konsulin.FinanceEmail,
			},
		},
	}

	// 8) Call OY with timeout
	ctxTimeout, cancel := context.WithTimeout(ctx, time.Duration(uc.InternalConfig.App.PaymentGatewayRequestTimeoutInSeconds)*time.Second)
	defer cancel()
	oyResp, err := uc.PaymentGateway.CreatePaymentRouting(ctxTimeout, oyReq)
	if err != nil {
		return nil, err
	}

	// 9) Build response
	return &responses.CreatePayResponse{
		PaymentCheckoutURL: oyResp.PaymentInfo.PaymentCheckoutURL,
		PartnerTrxID:       partnerTrxID,
		TrxID:              oyResp.TrxID,
		Amount:             amount,
	}, nil
}

func (uc *paymentUsecase) getBasePriceFromRoles(roles []string) (int, error) {
	for _, r := range roles {
		switch strings.ToLower(r) {
		case constvars.KonsulinRolePatient:
			return uc.InternalConfig.Pricing.PatientBasePrice, nil
		case constvars.KonsulinRolePractitioner:
			return uc.InternalConfig.Pricing.PractitionerBasePrice, nil
		case constvars.KonsulinRoleClinician:
			return uc.InternalConfig.Pricing.ClinicianBasePrice, nil
		case constvars.KonsulinRoleResearcher:
			return uc.InternalConfig.Pricing.ResearcherBasePrice, nil
		case constvars.KonsulinRoleClinicAdmin:
			return uc.InternalConfig.Pricing.ClinicAdminBasePrice, nil
		case constvars.KonsulinRoleSuperadmin:
			return uc.InternalConfig.Pricing.SuperadminBasePrice, nil
		}
	}
	return 0, fmt.Errorf("unsupported role for pricing")
}
