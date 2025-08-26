package payments

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/app/services/shared/storage"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/dto/responses"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/fhir_dto"
	"net/http"
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

	// 1) Early exit if status is not COMPLETE
	if constvars.OYPaymentRoutingStatus(request.PaymentStatus) != constvars.OYPaymentRoutingStatusComplete {
		uc.Log.Info("paymentUsecase.PaymentRoutingCallback non-complete status; ignoring",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String("payment_status", request.PaymentStatus),
		)
		return nil
	}

	// 2) Verify with OY (source of truth)
	verifyReq := &requests.OYCheckPaymentRoutingStatusRequest{PartnerTrxID: request.PartnerTrxID, SendCallback: false}
	ctxVerify, cancelVerify := context.WithTimeout(ctx, time.Duration(uc.InternalConfig.App.PaymentGatewayRequestTimeoutInSeconds)*time.Second)
	defer cancelVerify()
	verifyResp, err := uc.PaymentGateway.CheckPaymentRoutingStatus(ctxVerify, verifyReq)
	if err != nil {
		uc.Log.Error("paymentUsecase.PaymentRoutingCallback OY verify failed",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil
	}
	if constvars.OYPaymentRoutingStatus(verifyResp.PaymentStatus) != constvars.OYPaymentRoutingStatusComplete {
		uc.Log.Warn("paymentUsecase.PaymentRoutingCallback OY verify not complete; ignoring",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingOyPaymentStatusKey, verifyResp.PaymentStatus),
		)
		return nil
	}

	// 3) Parse partner_trx_id into id-version
	id, version, parseErr := parsePartnerTrxID(request.PartnerTrxID)
	if parseErr != nil {
		uc.Log.Error("paymentUsecase.PaymentRoutingCallback invalid partner_trx_id format",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String("partner_trx_id", request.PartnerTrxID),
			zap.Error(parseErr),
		)
		return nil
	}

	// 4) Fetch ServiceRequest specific version
	sr, err := uc.Storage.FhirClient.GetServiceRequestByIDAndVersion(ctx, id, version)
	if err != nil {
		uc.Log.Error("paymentUsecase.PaymentRoutingCallback failed fetching ServiceRequest",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil
	}

	// 5) Parse note.text -> NoteStorage
	note, err := extractNoteStorage(sr)
	if err != nil {
		uc.Log.Error("paymentUsecase.PaymentRoutingCallback failed parsing stored note",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil
	}

	// 6) POST instantiateURI with RawBody
	if err := callInstantiateURI(ctx, uc.Log, note.InstantiateURI, note.RawBody); err != nil {
		uc.Log.Error("paymentUsecase.PaymentRoutingCallback failed calling instantiate URI",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil
	}

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
	patientFullName := patients[0].FullName()

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
	loc := time.FixedZone("UTC+7", 7*60*60) // Force UTC+7 because OY is in UTC+7
	expiration := time.Now().In(loc).Add(time.Duration(uc.InternalConfig.App.PaymentExpiredTimeInMinutes) * time.Minute).Format("2006-01-02 15:04:05")

	uc.Log.Info("paymentUsecase.CreatePay expiration",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String("expiration", expiration),
	)

	oyReq := &requests.PaymentRequestDTO{
		PartnerUserID:           uid,
		UseLinkedAccount:        false,
		PartnerTransactionID:    partnerTrxID,
		NeedFrontend:            true,
		SenderEmail:             email,
		FullName:                patientFullName,
		PaymentExpirationTime:   expiration,
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

func parsePartnerTrxID(partnerTrxID string) (string, string, error) {
	parts := strings.Split(partnerTrxID, "-")
	if len(parts) < 2 {
		return "", "", fmt.Errorf("invalid partner_trx_id format")
	}
	version := parts[len(parts)-1]
	id := strings.Join(parts[:len(parts)-1], "-")
	if strings.TrimSpace(id) == "" || strings.TrimSpace(version) == "" {
		return "", "", fmt.Errorf("invalid partner_trx_id components")
	}
	return id, version, nil
}

func extractNoteStorage(sr *fhir_dto.GetServiceRequestOutput) (*requests.NoteStorage, error) {
	if sr == nil || len(sr.Note) == 0 || strings.TrimSpace(sr.Note[0].Text) == "" {
		return nil, fmt.Errorf("missing note storage payload")
	}
	var note requests.NoteStorage
	if err := json.Unmarshal([]byte(sr.Note[0].Text), &note); err != nil {
		return nil, err
	}
	return &note, nil
}

func callInstantiateURI(ctx context.Context, log *zap.Logger, url string, body json.RawMessage) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	req.Header.Set(constvars.HeaderContentType, constvars.MIMEApplicationJSON)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		log.Error("instantiate URI returned non-200",
			zap.Int("status_code", resp.StatusCode),
			zap.String("body", string(b)),
		)
		return fmt.Errorf("non-200 from instantiate uri: %d", resp.StatusCode)
	}
	return nil
}
