package payments

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/app/services/core/webhook"
	bundleSvc "konsulin-service/internal/app/services/fhir_spark/bundle"
	"konsulin-service/internal/app/services/shared/jwtmanager"
	"konsulin-service/internal/app/services/shared/storage"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/dto/responses"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/fhir_dto"
	"net/http"
	"net/url"
	"path"
	"slices"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

type paymentUsecase struct {
	TransactionRepository      contracts.TransactionRepository
	InternalConfig             *config.InternalConfig
	Log                        *zap.Logger
	JWTManager                 *jwtmanager.JWTManager
	PatientFhirClient          contracts.PatientFhirClient
	PractitionerFhirClient     contracts.PractitionerFhirClient
	PersonFhirClient           contracts.PersonFhirClient
	Storage                    *storage.ServiceRequestStorage
	PaymentGateway             contracts.PaymentGatewayService
	InvoiceFhirClient          contracts.InvoiceFhirClient
	PractitionerRoleFhirClient contracts.PractitionerRoleFhirClient
	SlotFhirClient             contracts.SlotFhirClient
	ScheduleFhirClient         contracts.ScheduleFhirClient
	BundleFhirClient           bundleSvc.BundleFhirClient
	SlotUsecase                contracts.SlotUsecaseIface
}

var (
	paymentUsecaseInstance contracts.PaymentUsecase
	oncePaymentUsecase     sync.Once
)

func NewPaymentUsecase(
	transactionRepository contracts.TransactionRepository,
	internalConfig *config.InternalConfig,
	jwtMgr *jwtmanager.JWTManager,
	patientFhirClient contracts.PatientFhirClient,
	practitionerFhirClient contracts.PractitionerFhirClient,
	personFhirClient contracts.PersonFhirClient,
	storageService *storage.ServiceRequestStorage,
	paymentGateway contracts.PaymentGatewayService,
	invoiceFhirClient contracts.InvoiceFhirClient,
	practitionerRoleFhirClient contracts.PractitionerRoleFhirClient,
	slotFhirClient contracts.SlotFhirClient,
	scheduleFhirClient contracts.ScheduleFhirClient,
	bundleFhirClient bundleSvc.BundleFhirClient,
	slotUsecase contracts.SlotUsecaseIface,
	logger *zap.Logger,
) contracts.PaymentUsecase {
	oncePaymentUsecase.Do(func() {
		instance := &paymentUsecase{
			TransactionRepository:      transactionRepository,
			InternalConfig:             internalConfig,
			Log:                        logger,
			JWTManager:                 jwtMgr,
			PatientFhirClient:          patientFhirClient,
			PractitionerFhirClient:     practitionerFhirClient,
			PersonFhirClient:           personFhirClient,
			Storage:                    storageService,
			PaymentGateway:             paymentGateway,
			InvoiceFhirClient:          invoiceFhirClient,
			PractitionerRoleFhirClient: practitionerRoleFhirClient,
			SlotFhirClient:             slotFhirClient,
			ScheduleFhirClient:         scheduleFhirClient,
			BundleFhirClient:           bundleFhirClient,
			SlotUsecase:                slotUsecase,
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

	// 6) Resolve instantiatesUri (prefer FHIR field, fallback to legacy note) and POST with RawBody
	uri, err := resolveInstantiatesURI(sr, note)
	if err != nil {
		uc.Log.Error("paymentUsecase.PaymentRoutingCallback failed resolving instantiatesUri",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil
	}
	if err := uc.callInstantiateURI(ctx, uri, note.RawBody); err != nil {
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

	// 1a) Validate and normalize service value (do not mutate request)
	requestedService, err := normalizeService(req.Service)
	if err != nil {
		return nil, err
	}

	// 1b) Enforce service access rule
	if !isServicePurchaseAllowed(requestedService, roles) {
		return nil, exceptions.ErrAuthInvalidRole(fmt.Errorf("role(s) not allowed to purchase service: %s", requestedService))
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

	// 4) Lookup resource identity by service (encapsulated)
	resourceID, displayFullName, err := uc.lookupIdentityByService(ctx, requestedService, email)
	if err != nil {
		return nil, err
	}
	partnerUserID := uid

	// 5) Determine ServiceRequest.subject
	subject := uc.determineServiceRequestSubject(requestedService, resourceID, roles)

	// 6) Build instantiateUri
	baseURL := strings.TrimRight(uc.InternalConfig.App.BaseUrl, "/")
	basePath := uc.InternalConfig.App.WebhookInstantiateBasePath
	instantiateURI := fmt.Sprintf("%s/%s/%s", baseURL, basePath, requestedService)
	// Normalize the URL to avoid duplicate slashes in the path
	if u, err := url.Parse(instantiateURI); err == nil {
		u.Path = path.Clean(u.Path)
		instantiateURI = u.String()
	}
	occurrence := time.Now().Format("2006-01-02T15:04:05-07:00")
	// Map service to requester resource type via specialized helper
	requesterResourceType := uc.mapServiceToRequesterResourceType(requestedService)

	storageOutput, err := uc.Storage.Create(ctx, &requests.CreateServiceRequestStorageInput{
		UID:             uid,
		ResourceType:    requesterResourceType,
		ID:              resourceID,
		Subject:         subject,
		InstantiatesUri: instantiateURI,
		RawBody:         req.Body,
		Occurrence:      occurrence,
	})
	if err != nil {
		uc.Log.Error("paymentUsecase.CreatePay failed storing ServiceRequest",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)

		return nil, err
	}
	partnerTrxID := storageOutput.PartnerTrxID

	// 6) Compute amount using service-based calculator
	amount, err := uc.calculateAmount(requestedService, req.TotalItem)
	if err != nil {
		return nil, err
	}

	// 7) Prepare OY payment request
	loc := time.FixedZone("UTC+7", 7*60*60) // Force UTC+7 because OY is in UTC+7
	expiration := time.Now().In(loc).Add(time.Duration(uc.InternalConfig.App.PaymentExpiredTimeInMinutes) * time.Minute).Format("2006-01-02 15:04:05")

	uc.Log.Info("paymentUsecase.CreatePay expiration",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String("expiration", expiration),
	)

	oyReq := &requests.PaymentRequestDTO{
		PartnerUserID:           partnerUserID,
		UseLinkedAccount:        false,
		PartnerTransactionID:    partnerTrxID,
		NeedFrontend:            true,
		SenderEmail:             email,
		FullName:                displayFullName,
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

// resolveInstantiatesURI returns the best-effort instantiates URI to be called.
// Preference order:
// 1) FHIR ServiceRequest.instantiatesUri (first element if present)
// 2) Legacy NoteStorage.InstantiateURI (deprecated)
// This function serves as a backward compatibility enabler during migration to the
// FHIR-native field. To remove backward compatibility, simply return sr.InstantiatesUri without checking note.InstantiateURI.
func resolveInstantiatesURI(sr *fhir_dto.GetServiceRequestOutput, note *requests.NoteStorage) (string, error) {
	if sr != nil && len(sr.InstantiatesUri) > 0 && strings.TrimSpace(sr.InstantiatesUri[0]) != "" {
		return sr.InstantiatesUri[0], nil
	}
	if note != nil && strings.TrimSpace(note.InstantiateURI) != "" {
		return note.InstantiateURI, nil
	}
	return "", fmt.Errorf("instantiatesUri not found in FHIR resource or legacy note")
}

func isServicePurchaseAllowed(service string, requesterRoles []string) bool {
	// Superadmin can purchase any service
	if hasRole(requesterRoles, constvars.KonsulinRoleSuperadmin) {
		return true
	}
	normalized := strings.ToLower(service)
	switch normalized {
	case string(constvars.ServiceAnalyze):
		return hasAnyRole(requesterRoles, []string{constvars.KonsulinRolePatient})
	case string(constvars.ServiceReport):
		return hasAnyRole(requesterRoles, []string{constvars.KonsulinRolePractitioner})
	case string(constvars.ServicePerformanceReport):
		return hasAnyRole(requesterRoles, []string{constvars.KonsulinRoleClinicAdmin})
	case string(constvars.ServiceAccessDataset):
		return hasAnyRole(requesterRoles, []string{constvars.KonsulinRoleResearcher})
	default:
		return false
	}
}

func hasAnyRole(roles []string, targets []string) bool {
	for _, t := range targets {
		if hasRole(roles, t) {
			return true
		}
	}
	return false
}

func hasRole(roles []string, target string) bool {
	for _, r := range roles {
		if strings.EqualFold(r, target) {
			return true
		}
	}
	return false
}

// normalizeService validates the service and returns its canonical value or an error (400-style) if invalid.
func normalizeService(service string) (string, error) {
	for _, known := range constvars.KnownServices {
		if strings.EqualFold(service, string(known)) {
			return string(known), nil
		}
	}
	return "", exceptions.ErrClientCustomMessage(fmt.Errorf("invalid service value: %s", service))
}

// calculateAmount validates service and totalItem against business rules and returns basePrice(service) * totalItem.
func (uc *paymentUsecase) calculateAmount(service string, totalItem int) (int, error) {
	serviceName := strings.ToLower(service)

	var (
		serviceType constvars.ServiceType
		basePrice   int
	)

	switch serviceName {
	case string(constvars.ServiceAnalyze):
		serviceType = constvars.ServiceAnalyze
		basePrice = uc.InternalConfig.ServicePricing.AnalyzeBasePrice
	case string(constvars.ServiceReport):
		serviceType = constvars.ServiceReport
		basePrice = uc.InternalConfig.ServicePricing.ReportBasePrice
	case string(constvars.ServicePerformanceReport):
		serviceType = constvars.ServicePerformanceReport
		basePrice = uc.InternalConfig.ServicePricing.PerformanceReportBasePrice
	case string(constvars.ServiceAccessDataset):
		serviceType = constvars.ServiceAccessDataset
		basePrice = uc.InternalConfig.ServicePricing.AccessDatasetBasePrice
	default:
		return 0, exceptions.ErrClientCustomMessage(fmt.Errorf("invalid service: %s", service))
	}

	minQty, ok := constvars.ServiceToMinQuantity[serviceType]
	if !ok {
		return 0, exceptions.ErrClientCustomMessage(fmt.Errorf("unsupported service: %s", service))
	}
	if totalItem < int(minQty) {
		return 0, exceptions.ErrClientCustomMessage(fmt.Errorf("total_item must be >= %d for service %s", int(minQty), serviceName))
	}

	amount := basePrice * totalItem
	return amount, nil
}

func (uc *paymentUsecase) callInstantiateURI(ctx context.Context, url string, body json.RawMessage) error {
	uc.Log.Info("paymentUsecase.callInstantiateURI request",
		zap.String("instantiate_uri", url),
		zap.String("body", string(body)),
	)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	req.Header.Set(constvars.HeaderContentType, constvars.MIMEApplicationJSON)
	// Attach internal forwarded JWT so webhook endpoint can bypass external auth
	if uc.JWTManager != nil {
		if out, err := uc.JWTManager.CreateToken(ctx, &jwtmanager.CreateTokenInput{Subject: webhook.PAYMENT_SERVICE_SUB}); err == nil {
			req.Header.Set(webhook.JWTForwardedFromPaymentServiceHeader, out.Token)
		}
	}
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		b, _ := io.ReadAll(resp.Body)
		uc.Log.Error("instantiate URI returned non-202",
			zap.Int("status_code", resp.StatusCode),
			zap.String("body", string(b)),
		)
		return fmt.Errorf("non-202 from instantiate uri: %d", resp.StatusCode)
	}
	return nil
}

// determineServiceRequestSubject returns the FHIR subject reference string based on service and roles.
// For Patient service, it returns "Patient/<patient-id>". For others, it maps to configured Group subjects.
func (uc *paymentUsecase) determineServiceRequestSubject(service string, patientID string, roles []string) string {
	normalized := strings.ToLower(service)
	switch normalized {
	case string(constvars.ServiceAnalyze):
		return fmt.Sprintf("%s/%s", constvars.ResourcePatient, patientID)
	case string(constvars.ServiceReport):
		return string(constvars.ServiceRequestSubjectPractitioner)
	case string(constvars.ServicePerformanceReport):
		return string(constvars.ServiceRequestSubjectClinicAdmin)
	case string(constvars.ServiceAccessDataset):
		return string(constvars.ServiceRequestSubjectResearcher)
	default:
		return string(constvars.ServiceRequestSubjectGuest)
	}
}

// lookupIdentityByService fetches resource identity based on the service and returns (resourceID, fullName).
// For analyze, it returns Patient ID; for report, Practitioner ID; for performance report and access dataset, Person ID.
func (uc *paymentUsecase) lookupIdentityByService(ctx context.Context, service string, email string) (string, string, error) {
	switch service {
	case string(constvars.ServiceAnalyze):
		patients, err := uc.PatientFhirClient.FindPatientByEmail(ctx, email)
		if err != nil {
			return "", "", err
		}
		if len(patients) == 0 {
			return "", "", exceptions.ErrUserNotExist(fmt.Errorf("no patient found"))
		}
		return patients[0].ID, patients[0].FullName(), nil
	case string(constvars.ServiceReport):
		practitioners, err := uc.PractitionerFhirClient.FindPractitionerByEmail(ctx, email)
		if err != nil {
			return "", "", err
		}
		if len(practitioners) == 0 {
			return "", "", exceptions.ErrUserNotExist(fmt.Errorf("no practitioner found"))
		}
		return practitioners[0].ID, practitioners[0].FullName(), nil
	case string(constvars.ServicePerformanceReport), string(constvars.ServiceAccessDataset):
		people, err := uc.PersonFhirClient.FindPersonByEmail(ctx, email)
		if err != nil {
			return "", "", err
		}
		if len(people) == 0 {
			return "", "", exceptions.ErrUserNotExist(fmt.Errorf("no person found"))
		}
		return people[0].ID, people[0].FullName(), nil
	default:
		return "", "", exceptions.ErrClientCustomMessage(fmt.Errorf("unsupported service: %s", service))
	}
}

// mapServiceToRequesterResourceType returns the FHIR requester resource type for a given service.
// analyze -> Patient, report -> Practitioner, performance-report/access-dataset -> Person, default empty.
func (uc *paymentUsecase) mapServiceToRequesterResourceType(service string) string {
	switch strings.ToLower(service) {
	case string(constvars.ServiceAnalyze):
		return constvars.ResourcePatient
	case string(constvars.ServiceReport):
		return constvars.ResourcePractitioner
	case string(constvars.ServicePerformanceReport), string(constvars.ServiceAccessDataset):
		return constvars.ResourcePerson
	default:
		return ""
	}
}

func (uc *paymentUsecase) HandleAppointmentPayment(
	ctx context.Context,
	req *requests.AppointmentPaymentRequest,
) (*responses.AppointmentPaymentResponse, error) {
	if !uc.whitelistAccessByRoles(ctx, []string{constvars.KonsulinRolePatient}) {
		return nil, exceptions.ErrAuthInvalidRole(errors.New("forbidden access"))
	}

	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	uc.Log.Info("paymentUsecase.HandleAppointmentPayment called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	if req.UseOnlinePayment {
		uc.Log.Warn("paymentUsecase.HandleAppointmentPayment online payment requested but not implemented",
			zap.String(constvars.LoggingRequestIDKey, requestID),
		)
		return nil, exceptions.BuildNewCustomError(
			nil,
			constvars.StatusNotImplemented,
			constvars.OnlinePaymentNotImplementedMessage,
			"online payment feature is not yet implemented",
		)
	}

	precond, err := uc.ensurePreconditionsValid(ctx, req)
	if err != nil {
		uc.Log.Error("paymentUsecase.HandleAppointmentPayment precondition validation failed",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, err
	}

	practitionerID := strings.TrimPrefix(precond.PractitionerRole.Practitioner.Reference, "Practitioner/")
	allPractitionerRoles, err := uc.PractitionerRoleFhirClient.Search(ctx, contracts.PractitionerRoleSearchParams{
		PractitionerID: practitionerID,
	})
	if err != nil {
		uc.Log.Error("paymentUsecase.HandleAppointmentPayment failed to fetch practitioner roles",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String("practitionerId", practitionerID),
			zap.Error(err),
		)
		return nil, exceptions.BuildNewCustomError(
			err,
			constvars.StatusInternalServerError,
			"failed to fetch practitioner roles",
			"failed to fetch practitioner roles",
		)
	}

	uc.Log.Info("paymentUsecase.HandleAppointmentPayment found practitioner roles",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Int("role_count", len(allPractitionerRoles)),
	)

	release, lockErr := uc.SlotUsecase.AcquireLocksForAppointment(
		ctx,
		allPractitionerRoles,
		precond.Slot.Start,
		precond.Slot.End,
		30*time.Second,
	)
	if lockErr != nil {
		uc.Log.Error("paymentUsecase.HandleAppointmentPayment failed to acquire locks",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(lockErr),
		)
		return nil, exceptions.BuildNewCustomError(
			lockErr,
			constvars.StatusConflict,
			"Unable to acquire necessary locks for booking. Please try again.",
			"lock acquisition failed",
		)
	}
	defer release(ctx)

	slotID := strings.TrimPrefix(req.SlotID, "Slot/")
	revalidatedSlot, slotErr := uc.SlotFhirClient.FindSlotByID(ctx, slotID)
	if slotErr != nil {
		uc.Log.Error("paymentUsecase.HandleAppointmentPayment failed to re-fetch slot",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(slotErr),
		)
		return nil, exceptions.BuildNewCustomError(
			slotErr,
			constvars.StatusInternalServerError,
			"failed to re-fetch slot",
			"failed to re-fetch slot",
		)
	}

	if revalidatedSlot.Status != fhir_dto.SlotStatusFree {
		uc.Log.Warn("paymentUsecase.HandleAppointmentPayment slot no longer free",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String("slotId", slotID),
			zap.String("status", string(revalidatedSlot.Status)),
		)
		return nil, exceptions.BuildNewCustomError(
			nil,
			constvars.StatusConflict,
			constvars.SlotNoLongerAvailableMessage,
			fmt.Sprintf("slot %s has status %s", slotID, revalidatedSlot.Status),
		)
	}

	bundleEntries, appointmentID, paymentNoticeID, err := uc.buildAppointmentPaymentBundle(ctx, req, precond, allPractitionerRoles)
	if err != nil {
		uc.Log.Error("paymentUsecase.HandleAppointmentPayment failed to build bundle",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, err
	}

	// atomic bundle transaction
	bundle := map[string]any{
		"resourceType": "Bundle",
		"type":         "transaction",
		"entry":        bundleEntries,
	}

	_, bundleErr := uc.BundleFhirClient.PostTransactionBundle(ctx, bundle)
	if bundleErr != nil {
		uc.Log.Error("paymentUsecase.HandleAppointmentPayment bundle execution failed",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(bundleErr),
		)
		return nil, exceptions.BuildNewCustomError(
			bundleErr,
			constvars.StatusInternalServerError,
			"Failed to process appointment booking. Please try again.",
			"FHIR bundle transaction failed",
		)
	}

	// best effort webhook notification
	asyncCtx := context.WithoutCancel(ctx)
	go uc.notifyProviderAsync(asyncCtx, notifyProviderAsyncInput{
		patient:       precond.Patient,
		paymentDate:   time.Now().Format(time.RFC3339),
		timeSlotStart: precond.Slot.Start.Format(time.RFC3339),
		timeSlotEnd:   precond.Slot.End.Format(time.RFC3339),
		amount:        formatMoney(precond.Invoice.TotalNet),
		amountPaid:    "0", // because for now only offline payment is supported
	})

	uc.Log.Info("paymentUsecase.HandleAppointmentPayment succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String("appointmentId", appointmentID),
	)

	return &responses.AppointmentPaymentResponse{
		Status:          constvars.StatusCreated,
		Message:         constvars.AppointmentPaymentSuccessMessage,
		AppointmentID:   fmt.Sprintf("%s/%s", constvars.ResourceAppointment, appointmentID),
		SlotID:          req.SlotID,
		PaymentNoticeID: fmt.Sprintf("%s/%s", constvars.ResourcePaymentNotice, paymentNoticeID),
	}, nil
}

func (uc *paymentUsecase) whitelistAccessByRoles(ctx context.Context, allowedRoles []string) bool {
	roles, _ := ctx.Value(constvars.CONTEXT_FHIR_ROLE).([]string)

	for _, role := range roles {
		if slices.Contains(allowedRoles, role) {
			return true
		}
	}

	return false
}

// resourceFetchError is used to preserve which resource failed during concurrent fetches.
type resourceFetchError struct {
	resource string
	err      error
}

func (e *resourceFetchError) Error() string { return e.resource + ": " + e.err.Error() }
func (e *resourceFetchError) Unwrap() error { return e.err }

// preconditionData holds all fetched resources needed for appointment payment
type preconditionData struct {
	Slot             *fhir_dto.Slot
	PractitionerRole *fhir_dto.PractitionerRole
	Practitioner     *fhir_dto.Practitioner
	Patient          *fhir_dto.Patient
	Invoice          *fhir_dto.Invoice
	Schedule         *fhir_dto.Schedule
}

// ensurePreconditionsValid fetches and validates all required resources
func (uc *paymentUsecase) ensurePreconditionsValid(
	ctx context.Context,
	req *requests.AppointmentPaymentRequest,
) (*preconditionData, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	uid, _ := ctx.Value(constvars.CONTEXT_UID).(string)

	slotID := strings.TrimPrefix(req.SlotID, "Slot/")
	practitionerRoleID := strings.TrimPrefix(req.PractitionerRoleID, "PractitionerRole/")
	patientID := strings.TrimPrefix(req.PatientID, "Patient/")
	invoiceID := strings.TrimPrefix(req.InvoiceID, "Invoice/")

	// Concurrent fetches with early cancellation on first error
	g, gctx := errgroup.WithContext(ctx)

	var (
		fetchedSlot             *fhir_dto.Slot
		fetchedPractitionerRole *fhir_dto.PractitionerRole
		fetchedPatient          *fhir_dto.Patient
		fetchedInvoices         []fhir_dto.Invoice
		schedules               []fhir_dto.Schedule
		schedulesErr            error
	)

	g.Go(func() error {
		s, err := uc.SlotFhirClient.FindSlotByID(gctx, slotID)
		if err != nil {
			return &resourceFetchError{resource: "slot", err: err}
		}
		fetchedSlot = s
		return nil
	})

	g.Go(func() error {
		pr, err := uc.PractitionerRoleFhirClient.FindPractitionerRoleByID(gctx, practitionerRoleID)
		if err != nil {
			return &resourceFetchError{resource: "practitionerRole", err: err}
		}
		fetchedPractitionerRole = pr
		return nil
	})

	g.Go(func() error {
		p, err := uc.PatientFhirClient.FindPatientByID(gctx, patientID)
		if err != nil {
			return &resourceFetchError{resource: "patient", err: err}
		}
		match := false
		for _, identifier := range p.Identifier {
			if identifier.Value == uid {
				match = true
				break
			}
		}
		if !match {
			return &resourceFetchError{resource: "patient", err: errors.New("patient ID does not match with the current user")}
		}
		fetchedPatient = p
		return nil
	})

	g.Go(func() error {
		inv, err := uc.InvoiceFhirClient.Search(gctx, contracts.InvoiceSearchParams{ID: invoiceID})
		if err != nil {
			return &resourceFetchError{resource: "invoice", err: err}
		}
		fetchedInvoices = inv
		return nil
	})

	g.Go(func() error {
		sched, err := uc.ScheduleFhirClient.FindScheduleByPractitionerRoleID(gctx, practitionerRoleID)
		if err != nil {
			schedulesErr = err
			return nil
		}
		schedules = sched
		return nil
	})

	if err := g.Wait(); err != nil {
		resType := "unknown"
		unwrapped := err
		if fe, ok := err.(*resourceFetchError); ok {
			resType = fe.resource
			unwrapped = fe.err
		}
		uc.Log.Error("paymentUsecase.ensurePreconditionsValid failed to fetch resource",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String("resourceType", resType),
			zap.Error(unwrapped),
		)
		return nil, exceptions.BuildNewCustomError(
			unwrapped,
			constvars.StatusBadRequest,
			unwrapped.Error(),
			"precondition checks failed",
		)
	}

	if len(fetchedInvoices) != 1 {
		return nil, exceptions.BuildNewCustomError(
			nil,
			constvars.StatusNotFound,
			"Invoice not found or multiple invoices found",
			fmt.Sprintf("invoice %s not found or multiple invoices found", invoiceID),
		)
	}

	isInvoiceBelongsToPractitionerRole := slices.ContainsFunc(fetchedInvoices[0].Participant, func(p fhir_dto.InvoiceParticipant) bool {
		return p.Actor.Reference == req.PractitionerRoleID
	})

	if !isInvoiceBelongsToPractitionerRole {
		return nil, exceptions.BuildNewCustomError(
			nil,
			constvars.StatusBadRequest,
			"Invoice does not belong to the specified practitioner role",
			fmt.Sprintf("invoice %s does not belong to the specified practitioner role", invoiceID),
		)
	}

	if fetchedSlot.Status != fhir_dto.SlotStatusFree {
		return nil, exceptions.BuildNewCustomError(
			nil,
			constvars.StatusConflict,
			constvars.SlotNoLongerAvailableMessage,
			fmt.Sprintf("slot %s has status %s, expected free", slotID, fetchedSlot.Status),
		)
	}

	scheduleRef := fetchedSlot.Schedule.Reference
	if schedulesErr != nil || len(schedules) == 0 {
		return nil, exceptions.BuildNewCustomError(
			schedulesErr,
			constvars.StatusBadRequest,
			"Failed to validate slot ownership",
			"failed to find schedule for practitioner role",
		)
	}

	matchFound := false
	var schedule *fhir_dto.Schedule
	for i := range schedules {
		if "Schedule/"+schedules[i].ID == scheduleRef {
			matchFound = true
			schedule = &schedules[i]
			break
		}
	}

	if !matchFound {
		return nil, exceptions.BuildNewCustomError(
			nil,
			constvars.StatusBadRequest,
			"Slot does not belong to the specified practitioner role",
			fmt.Sprintf("slot schedule %s does not match practitioner role", scheduleRef),
		)
	}

	nowLocal := time.Now().In(fetchedSlot.Start.Location())
	if fetchedSlot.Start.Before(nowLocal) || fetchedSlot.Start.Equal(nowLocal) {
		return nil, exceptions.BuildNewCustomError(
			nil,
			constvars.StatusBadRequest,
			constvars.SlotInPastMessage,
			fmt.Sprintf("slot start time %s is not in the future", fetchedSlot.Start.Format(time.RFC3339)),
		)
	}

	practitionerRef := fetchedPractitionerRole.Practitioner.Reference
	practitionerID := strings.TrimPrefix(practitionerRef, "Practitioner/")
	practitioner, practErr := uc.PractitionerFhirClient.FindPractitionerByID(ctx, practitionerID)
	if practErr != nil {
		return nil, exceptions.BuildNewCustomError(
			practErr,
			constvars.StatusInternalServerError,
			"failed to fetch practitioner",
			"failed to fetch practitioner",
		)
	}

	return &preconditionData{
		Slot:             fetchedSlot,
		PractitionerRole: fetchedPractitionerRole,
		Practitioner:     practitioner,
		Patient:          fetchedPatient,
		Invoice:          &fetchedInvoices[0],
		Schedule:         schedule,
	}, nil
}

// buildAppointmentPaymentBundle constructs the full transaction bundle entries and
// returns the entries alongside deterministically generated IDs for Appointment and PaymentNotice.
// duplicate helper definitions removed (see appointment_payment_helpers.go)
