package payments

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"path"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/app/services/shared/jwtmanager"
	"konsulin-service/internal/app/services/shared/storage"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/dto/responses"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/fhir_dto"
	"konsulin-service/internal/pkg/fhir_http_client"

	bundleSvc "konsulin-service/internal/app/services/fhir_spark/bundle"

	xendit "github.com/xendit/xendit-go/v7"
	common "github.com/xendit/xendit-go/v7/common"
	xinvoice "github.com/xendit/xendit-go/v7/invoice"
	"go.uber.org/zap"
)

type paymentUsecase struct {
	TransactionRepository      contracts.TransactionRepository
	InternalConfig             *config.InternalConfig
	Log                        *zap.Logger
	JWTManager                 *jwtmanager.JWTManager
	PatientFhirClient          contracts.PatientFhirClient
	PractitionerFhirClient     contracts.PractitionerFhirClient
	Storage                    *storage.ServiceRequestStorage
	InvoiceFhirClient          contracts.InvoiceFhirClient
	PractitionerRoleFhirClient contracts.PractitionerRoleFhirClient
	SlotFhirClient             contracts.SlotFhirClient
	ScheduleFhirClient         contracts.ScheduleFhirClient
	BundleFhirClient           bundleSvc.BundleFhirClient
	SlotUsecase                contracts.SlotUsecaseIface
	XenditClient               *xendit.APIClient
	FHIRClient                 appointmentResourceClient
	// CreateAppointmentInvoice is the injectable seam for the Xendit invoice
	// creation call made by handleAppointmentBooking; when nil it falls back to
	// createXenditInvoiceForAppointment. Tests stub it to exercise the booking
	// flow offline without a Xendit sandbox.
	CreateAppointmentInvoice func(ctx context.Context, req *requests.AppointmentPaymentRequest, precond *preconditionData) (string, error)
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
	storageService *storage.ServiceRequestStorage,
	xenditClient *xendit.APIClient,
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
			Storage:                    storageService,
			InvoiceFhirClient:          invoiceFhirClient,
			PractitionerRoleFhirClient: practitionerRoleFhirClient,
			SlotFhirClient:             slotFhirClient,
			ScheduleFhirClient:         scheduleFhirClient,
			BundleFhirClient:           bundleFhirClient,
			SlotUsecase:                slotUsecase,
			XenditClient:               xenditClient,
			FHIRClient:                 fhir_http_client.New(logger),
		}
		paymentUsecaseInstance = instance
	})
	return paymentUsecaseInstance
}

// handleAppointmentPaymentExpired handles an EXPIRED callback for appointment payments.
// It deletes the slot, invoice, and appointment resources atomically to free the
// time slot and clean up the unconfirmed booking.
func (uc *paymentUsecase) handleAppointmentPaymentExpired(
	ctx context.Context,
	fields appointmentExternalIDFields,
	_ *fhir_dto.Slot,
) error {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)

	// Delete slot, invoice, and appointment via one atomic FHIR transaction bundle
	bundle := map[string]any{
		constvars.FhirFieldResourceType: "Bundle",
		"type":                          "transaction",
		"entry": []map[string]any{
			{
				constvars.FhirFieldRequest: map[string]any{
					constvars.FhirFieldMethod: constvars.MethodDelete,
					constvars.FhirFieldURL:    constvars.ResourceSlot + "/" + fields.SlotID,
				},
			},
			{
				constvars.FhirFieldRequest: map[string]any{
					constvars.FhirFieldMethod: constvars.MethodDelete,
					constvars.FhirFieldURL:    constvars.ResourceInvoice + "/" + fields.InvoiceID,
				},
			},
			{
				constvars.FhirFieldRequest: map[string]any{
					constvars.FhirFieldMethod: constvars.MethodDelete,
					constvars.FhirFieldURL:    constvars.ResourceAppointment + "/" + fields.AppointmentID,
				},
			},
		},
	}
	if _, err := uc.BundleFhirClient.PostTransactionBundle(ctx, bundle); err != nil {
		uc.Log.Error("paymentUsecase.handleAppointmentPaymentExpired failed to delete expired resources via bundle",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String("slotId", fields.SlotID),
			zap.Error(err),
		)
		return exceptions.BuildNewCustomError(
			err,
			constvars.StatusInternalServerError,
			"Failed to delete expired appointment resources",
			"failed to delete slot, invoice, and appointment via bundle",
		)
	}

	uc.Log.Info("paymentUsecase.handleAppointmentPaymentExpired deleted slot, invoice, and appointment",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String("slotId", fields.SlotID),
		zap.String("invoiceId", fields.InvoiceID),
		zap.String("appointmentId", fields.AppointmentID),
	)
	return nil
}

func (uc *paymentUsecase) CreatePay(ctx context.Context, req *requests.CreatePayRequest) (*responses.CreatePayResponse, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	uc.Log.Info("paymentUsecase.CreatePay called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	// 1) Reject guest role
	roles, _ := ctx.Value(constvars.CONTEXT_FHIR_ROLE).([]string)
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
	uid, _ := ctx.Value(constvars.CONTEXT_UID).(string)

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

	// 5) Determine ServiceRequest.subject
	subject := determineServiceRequestSubject(requestedService, resourceID)

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
	requesterResourceType := mapServiceToRequesterResourceType(requestedService)

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
	amount, basePrice, err := uc.calculateAmount(requestedService, req.TotalItem)
	if err != nil {
		return nil, err
	}

	// 7) Create Xendit invoice
	if uc.XenditClient == nil {
		return nil, exceptions.ErrServerProcess(fmt.Errorf("xendit client not initialized"))
	}

	desc := fmt.Sprintf("pembayaran layanan %s dari konsulin sejumlah %d item", req.Service, req.TotalItem)
	durationSeconds := float32(uc.InternalConfig.App.PaymentExpiredTimeInMinutes * 60)

	externalID := fmt.Sprintf("%s:%s", constvars.WebhookPaymentService, partnerTrxID)
	invoiceReq := xinvoice.NewCreateInvoiceRequest(externalID, float64(amount))
	invoiceReq.SetCurrency(constvars.CurrencyIndonesianRupiah)
	invoiceReq.SetDescription(desc)
	invoiceReq.SetSuccessRedirectUrl(uc.InternalConfig.App.FrontendDomain)
	invoiceReq.SetFailureRedirectUrl(uc.InternalConfig.App.FrontendDomain)
	if durationSeconds > 0 {
		invoiceReq.SetInvoiceDuration(durationSeconds)
	}

	customer := xinvoice.NewCustomerObject()
	customer.SetGivenNames(displayFullName)
	customer.SetEmail(email)
	invoiceReq.SetCustomer(*customer)

	notif := xinvoice.NewNotificationPreference()
	notif.SetInvoiceCreated([]xinvoice.NotificationChannel{xinvoice.NOTIFICATIONCHANNEL_EMAIL})
	notif.SetInvoicePaid([]xinvoice.NotificationChannel{xinvoice.NOTIFICATIONCHANNEL_EMAIL})
	invoiceReq.SetCustomerNotificationPreference(*notif)

	item := xinvoice.NewInvoiceItem(requestedService, float32(basePrice), float32(req.TotalItem))
	invoiceReq.SetItems([]xinvoice.InvoiceItem{*item})

	// 8) Execute with timeout
	ctxTimeout, cancel := context.WithTimeout(ctx, time.Duration(uc.InternalConfig.App.PaymentGatewayRequestTimeoutInSeconds)*time.Second)
	defer cancel()
	apiReq := uc.XenditClient.InvoiceApi.CreateInvoice(ctxTimeout).CreateInvoiceRequest(*invoiceReq)
	inv, httpResp, xenditErr := apiReq.Execute()
	if xenditErr != nil {
		return nil, uc.mapXenditError(ctx, xenditErr, httpResp)
	}

	// 9) Build response from Xendit invoice
	return &responses.CreatePayResponse{
		PaymentCheckoutURL: inv.GetInvoiceUrl(),
		PartnerTrxID:       partnerTrxID,
		TrxID:              inv.GetId(),
		Amount:             amount,
	}, nil
}

func (uc *paymentUsecase) mapXenditError(ctx context.Context, err *common.XenditSdkError, httpResp *http.Response) *exceptions.CustomError {
	logXenditErrorResponse(ctx, uc.Log, httpResp)

	statusCode := extractStatusCodeFromXenditError(err, httpResp)
	rawMsg := extractMessageFromXenditError(err)
	devMsg := fmt.Sprintf("xendit error code=%s message=%s", err.ErrorCode(), rawMsg)
	wrappedErr := errors.New(devMsg)

	if err.ErrorCode() == "API_VALIDATION_ERROR" || statusCode == http.StatusBadRequest {
		return exceptions.BuildNewCustomError(wrappedErr, constvars.StatusBadRequest, rawMsg, devMsg)
	}
	return exceptions.BuildNewCustomError(wrappedErr, statusCode, rawMsg, devMsg)
}

// logXenditErrorResponse logs the Xendit error response body if available.
func logXenditErrorResponse(ctx context.Context, log *zap.Logger, httpResp *http.Response) {
	if httpResp == nil || httpResp.Body == nil {
		return
	}
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	bodyBytes, readErr := io.ReadAll(httpResp.Body)
	if readErr == nil && len(bodyBytes) > 0 {
		log.Error("paymentUsecase.mapXenditError Xendit error response body",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String("response_body", string(bodyBytes)),
			zap.Int("status_code", httpResp.StatusCode),
		)
	}
}

// extractStatusCodeFromXenditError determines HTTP status from Xendit SDK error or HTTP response.
func extractStatusCodeFromXenditError(err *common.XenditSdkError, httpResp *http.Response) int {
	if httpResp != nil && httpResp.StatusCode > 0 {
		return httpResp.StatusCode
	}
	if statusText := strings.TrimSpace(err.Status()); statusText != "" {
		if parsed, convErr := strconv.Atoi(statusText); convErr == nil {
			return parsed
		}
	}
	return constvars.StatusInternalServerError
}

// extractMessageFromXenditError extracts a user-facing message from a Xendit error.
func extractMessageFromXenditError(err *common.XenditSdkError) string {
	rawMsg := strings.TrimSpace(err.Error())
	if raw := err.RawResponse(); raw != nil {
		if messageAny, ok := raw["message"]; ok {
			if msgStr, ok := messageAny.(string); ok {
				if trimmed := strings.TrimSpace(msgStr); trimmed != "" {
					rawMsg = trimmed
				}
			}
		}
	}
	if rawMsg == "" {
		return constvars.ErrClientCannotProcessRequest
	}
	return rawMsg
}

// createXenditInvoiceForAppointment creates a Xendit invoice for appointment payment
func (uc *paymentUsecase) createXenditInvoiceForAppointment(
	ctx context.Context,
	req *requests.AppointmentPaymentRequest,
	precond *preconditionData,
) (string, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)

	slotID := strings.TrimPrefix(req.SlotID, "Slot/")
	practitionerRoleID := strings.TrimPrefix(req.PractitionerRoleID, constvars.FHIRRefPrefixPractitionerRole)
	patientID := strings.TrimPrefix(req.PatientID, constvars.FHIRRefPrefixPatient)
	invoiceID := strings.TrimPrefix(req.InvoiceID, constvars.FHIRRefPrefixInvoice)
	healthcareServiceID := strings.TrimPrefix(req.HealthcareServiceID, constvars.FHIRRefPrefixHealthcareService)
	appointmentID := strings.TrimPrefix(req.AppointmentID, constvars.ResourceAppointment+"/")
	// Format: appointment:{slotID}:{practitionerRoleID}:{patientID}:{invoiceID}:{healthcareServiceID}:{appointmentID}
	externalID := fmt.Sprintf("%s:%s:%s:%s:%s:%s:%s",
		constvars.AppointmentPaymentService,
		slotID,
		practitionerRoleID,
		patientID,
		invoiceID,
		healthcareServiceID,
		appointmentID,
	)

	dateOnly := precond.Slot.Start.Format(time.DateOnly)
	startTime := precond.Slot.Start.Format(time.TimeOnly)
	endTime := precond.Slot.End.Format(time.TimeOnly)
	description := fmt.Sprintf("Pembayaran janji temu pada tanggal %s pukul %s - %s", dateOnly, startTime, endTime)

	amount := int(math.Ceil(precond.Invoice.TotalNet.Value))

	patientEmails := precond.Patient.GetEmailAddresses()
	var patientEmail string
	if len(patientEmails) > 0 {
		patientEmail = patientEmails[0]
	}

	patientPhoneNumbers := precond.Patient.GetPhoneNumbers()
	var patientPhoneNumber string
	if len(patientPhoneNumbers) > 0 {
		patientPhoneNumber = patientPhoneNumbers[0]
	}

	patientName := precond.Patient.FullName()
	if patientName == "" {
		patientName = "Pasien Konsulin"
	}

	durationSeconds := float32(uc.InternalConfig.App.PaymentExpiredTimeInMinutes * 60)

	invoiceReq := xinvoice.NewCreateInvoiceRequest(externalID, float64(amount))
	invoiceReq.SetCurrency(constvars.CurrencyIndonesianRupiah)
	invoiceReq.SetDescription(description)
	invoiceReq.SetSuccessRedirectUrl(uc.InternalConfig.App.FrontendDomain)
	invoiceReq.SetFailureRedirectUrl(uc.InternalConfig.App.FrontendDomain)
	if durationSeconds > 0 {
		invoiceReq.SetInvoiceDuration(durationSeconds)
	}

	customer := xinvoice.NewCustomerObject()
	customer.SetGivenNames(patientName)

	var preferredNotificationChannel []xinvoice.NotificationChannel

	if patientEmail != "" {
		customer.SetEmail(patientEmail)
		preferredNotificationChannel = append(
			preferredNotificationChannel,
			xinvoice.NOTIFICATIONCHANNEL_EMAIL,
		)
	}

	if patientPhoneNumber != "" {
		customer.SetMobileNumber(patientPhoneNumber)
		customer.SetPhoneNumber(patientPhoneNumber)

		preferredNotificationChannel = append(
			preferredNotificationChannel,
			xinvoice.NOTIFICATIONCHANNEL_SMS,
			xinvoice.NOTIFICATIONCHANNEL_WHATSAPP,
		)
	}

	invoiceReq.SetCustomer(*customer)

	if len(preferredNotificationChannel) != 0 {
		notif := xinvoice.NewNotificationPreference()
		notif.SetInvoiceCreated(preferredNotificationChannel)
		notif.SetInvoicePaid(preferredNotificationChannel)
		notif.SetInvoiceReminder(preferredNotificationChannel)

		invoiceReq.SetCustomerNotificationPreference(*notif)
	}

	item := xinvoice.NewInvoiceItem("Pembayaran Janji Temu", float32(amount), float32(1))
	invoiceReq.SetItems([]xinvoice.InvoiceItem{*item})

	ctxTimeout, cancel := context.WithTimeout(ctx, time.Duration(uc.InternalConfig.App.PaymentGatewayRequestTimeoutInSeconds)*time.Second)
	defer cancel()

	apiReq := uc.XenditClient.InvoiceApi.CreateInvoice(ctxTimeout).CreateInvoiceRequest(*invoiceReq)
	inv, httpResp, xenditErr := apiReq.Execute()
	if xenditErr != nil {
		uc.Log.Error("paymentUsecase.createXenditInvoiceForAppointment failed",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String("slotId", slotID),
			zap.Error(xenditErr),
		)
		return "", uc.mapXenditError(ctx, xenditErr, httpResp)
	}

	uc.Log.Info("paymentUsecase.createXenditInvoiceForAppointment succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String("slotId", slotID),
		zap.String("invoiceId", inv.GetId()),
		zap.String("externalId", externalID),
	)

	return inv.GetInvoiceUrl(), nil
}

// appointmentExternalIDFields holds all identifiers parsed from the Xendit external_id.
type appointmentExternalIDFields struct {
	SlotID              string
	PractitionerRoleID  string
	PatientID           string
	InvoiceID           string
	HealthcareServiceID string
	AppointmentID       string
}

// parseAppointmentExternalID parses the appointment external_id format:
//
//	appointment:{slotID}:{practitionerRoleID}:{patientID}:{invoiceID}:{healthcareServiceID}:{appointmentID}
func parseAppointmentExternalID(externalID string) (appointmentExternalIDFields, error) {
	parts := strings.Split(externalID, ":")
	if len(parts) != 7 {
		return appointmentExternalIDFields{}, fmt.Errorf("invalid appointment external_id format: expected 7 colon-separated parts, got %d", len(parts))
	}
	if parts[0] != string(constvars.AppointmentPaymentService) {
		return appointmentExternalIDFields{}, fmt.Errorf("invalid appointment external_id format: expected prefix '%s', got '%s'", constvars.AppointmentPaymentService, parts[0])
	}
	fields := appointmentExternalIDFields{
		SlotID:              parts[1],
		PractitionerRoleID:  parts[2],
		PatientID:           parts[3],
		InvoiceID:           parts[4],
		HealthcareServiceID: parts[5],
		AppointmentID:       parts[6],
	}
	if strings.TrimSpace(fields.SlotID) == "" ||
		strings.TrimSpace(fields.PractitionerRoleID) == "" ||
		strings.TrimSpace(fields.PatientID) == "" ||
		strings.TrimSpace(fields.InvoiceID) == "" ||
		strings.TrimSpace(fields.HealthcareServiceID) == "" ||
		strings.TrimSpace(fields.AppointmentID) == "" {
		return appointmentExternalIDFields{}, fmt.Errorf("invalid appointment external_id format: one or more fields are empty")
	}
	return fields, nil
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
func (uc *paymentUsecase) calculateAmount(service string, totalItem int) (int, int, error) {
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
		return 0, 0, exceptions.ErrClientCustomMessage(fmt.Errorf("invalid service: %s", service))
	}

	minQty, ok := constvars.ServiceToMinQuantity[serviceType]
	if !ok {
		return 0, 0, exceptions.ErrClientCustomMessage(fmt.Errorf("unsupported service: %s", service))
	}
	if totalItem < int(minQty) {
		return 0, 0, exceptions.ErrClientCustomMessage(fmt.Errorf("total_item must be >= %d for service %s", int(minQty), serviceName))
	}

	amount := basePrice * totalItem
	return amount, basePrice, nil
}

// determineServiceRequestSubject returns the FHIR subject reference string based on service.
// For Patient service, it returns "Patient/<patient-id>". For others, it maps to configured Group subjects.
func determineServiceRequestSubject(service string, patientID string) string {
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
		return resolveEmailIdentity(ctx, email, "no patient found", uc.PatientFhirClient.FindPatientByEmail,
			func(ps []fhir_dto.Patient) (string, string) { return ps[0].ID, ps[0].FullName() })
	case string(constvars.ServiceReport), string(constvars.ServicePerformanceReport), string(constvars.ServiceAccessDataset):
		return resolveEmailIdentity(ctx, email, "no practitioner found", uc.PractitionerFhirClient.FindPractitionerByEmail,
			func(ps []fhir_dto.Practitioner) (string, string) { return ps[0].ID, ps[0].FullName() })
	default:
		return "", "", exceptions.ErrClientCustomMessage(fmt.Errorf("unsupported service: %s", service))
	}
}

// resolveEmailIdentity looks up the FHIR resource matching email via find and
// returns its ID and display name, or ErrUserNotExist when no match is found.
// first extracts the identity from the first result.
func resolveEmailIdentity[T any](ctx context.Context, email, notFoundMsg string, find func(context.Context, string) ([]T, error), first func([]T) (string, string)) (string, string, error) {
	results, err := find(ctx, email)
	if err != nil {
		return "", "", err
	}
	if len(results) == 0 {
		return "", "", exceptions.ErrUserNotExist(errors.New(notFoundMsg))
	}
	id, name := first(results)
	return id, name, nil
}

// mapServiceToRequesterResourceType returns the FHIR requester resource type for a given service.
// analyze -> Patient, report -> Practitioner, performance-report/access-dataset -> Person, default empty.
func mapServiceToRequesterResourceType(service string) string {
	switch strings.ToLower(service) {
	case string(constvars.ServiceAnalyze):
		return constvars.ResourcePatient
	case string(constvars.ServiceReport):
		return constvars.ResourcePractitioner
	case string(constvars.ServicePerformanceReport), string(constvars.ServiceAccessDataset):
		return constvars.ResourcePractitioner
	default:
		return ""
	}
}

func (uc *paymentUsecase) HandleAppointmentPayment(
	ctx context.Context,
	req *requests.AppointmentPaymentRequest,
) (*responses.AppointmentPaymentResponse, error) {
	if !whitelistAccessByRoles(ctx, []string{constvars.KonsulinRolePatient, constvars.KonsulinRoleSuperadmin}) {
		return nil, exceptions.ErrAuthInvalidRole(errors.New("forbidden access"))
	}

	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	uc.Log.Info("paymentUsecase.HandleAppointmentPayment called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	precond, err := uc.ensurePreconditionsValid(ctx, req)
	if err != nil {
		uc.Log.Error("paymentUsecase.HandleAppointmentPayment precondition validation failed",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, err
	}

	practitionerID := strings.TrimPrefix(precond.PractitionerRole.Practitioner.Reference, constvars.FHIRRefPrefixPractitioner)
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
			constvars.ErrDevFailedToFetchPractitionerRoles,
			constvars.ErrDevFailedToFetchPractitionerRoles,
		)
	}

	uc.Log.Info("paymentUsecase.HandleAppointmentPayment found practitioner roles",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Int("role_count", len(allPractitionerRoles)),
	)

	release, lockErr := uc.acquireBookingLocks(ctx, precond, 30*time.Second)
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
	defer func() { release(context.Background()) }()

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

	// Overlap check: query non-free slots for the schedule and check for time conflicts
	overlapParams := contracts.SlotSearchParams{
		Start:  "lt" + revalidatedSlot.End.Format(time.RFC3339),
		End:    "gt" + revalidatedSlot.Start.Format(time.RFC3339),
		Status: fhir_dto.SlotStatus(string(fhir_dto.SlotStatusBusyUnavailable) + "," + string(fhir_dto.SlotStatusBusyTentative)),
	}
	scheduleID := strings.TrimPrefix(precond.Slot.Schedule.Reference, "Schedule/")
	overlappingSlots, searchErr := uc.SlotFhirClient.FindSlotsByScheduleWithQuery(ctx, scheduleID, overlapParams)
	if searchErr != nil {
		uc.Log.Error("paymentUsecase.HandleAppointmentPayment overlap search failed",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(searchErr),
		)
		return nil, exceptions.BuildNewCustomError(
			searchErr,
			constvars.StatusInternalServerError,
			"Failed to check slot availability. Please try again.",
			"overlap search failed",
		)
	}
	if overlap := getOverlappingNonFreeSlots(overlappingSlots, revalidatedSlot.Start, revalidatedSlot.End); len(overlap) > 0 {
		uc.Log.Warn("paymentUsecase.HandleAppointmentPayment overlapping non-free slot found",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String("slotId", slotID),
			zap.String("conflictingSlotId", overlap[0].ID),
		)
		return nil, exceptions.BuildNewCustomError(
			nil,
			constvars.StatusConflict,
			constvars.SlotNoLongerAvailableMessage,
			fmt.Sprintf("overlapping non-free slot %s with status %s", overlap[0].ID, overlap[0].Status),
		)
	}

	// Cross-role guard: the practitioner may hold other bookable roles (other locations).
	// Reject the booking if a non-free slot overlaps the window in any sibling schedule.
	crossOverlap, crossErr := uc.findCrossRoleOverlap(ctx, allPractitionerRoles, scheduleID, revalidatedSlot.Start, revalidatedSlot.End)
	if crossErr != nil {
		uc.Log.Error("paymentUsecase.HandleAppointmentPayment cross-role overlap search failed",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(crossErr),
		)
		return nil, exceptions.BuildNewCustomError(
			crossErr,
			constvars.StatusInternalServerError,
			"Failed to check slot availability. Please try again.",
			"cross-role overlap search failed",
		)
	}
	if crossOverlap != nil {
		uc.Log.Warn("paymentUsecase.HandleAppointmentPayment cross-role overlap found",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String("slotId", slotID),
			zap.String("conflictingSlotId", crossOverlap.ID),
			zap.String("conflictingStatus", string(crossOverlap.Status)),
		)
		return nil, exceptions.BuildNewCustomError(
			nil,
			constvars.StatusConflict,
			constvars.SlotNoLongerAvailableMessage,
			fmt.Sprintf("overlapping non-free slot %s with status %s in sibling schedule", crossOverlap.ID, crossOverlap.Status),
		)
	}

	appointment, apptErr := uc.fetchAppointment(ctx, req.AppointmentID)
	if apptErr != nil {
		uc.Log.Error("paymentUsecase.HandleAppointmentPayment failed to fetch appointment",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String("appointmentId", req.AppointmentID),
			zap.Error(apptErr),
		)
		var fhirErr *fhir_http_client.FHIRHTTPError
		if errors.As(apptErr, &fhirErr) && fhirErr.StatusCode == http.StatusNotFound {
			return nil, exceptions.BuildNewCustomError(
				apptErr,
				constvars.StatusNotFound,
				"Appointment not found",
				fmt.Sprintf("failed to fetch appointment %s", req.AppointmentID),
			)
		}
		return nil, exceptions.BuildNewCustomError(
			apptErr,
			constvars.StatusInternalServerError,
			"Failed to fetch appointment. Please try again.",
			fmt.Sprintf("failed to fetch appointment %s", req.AppointmentID),
		)
	}

	if validateErr := validateProposedAppointment(appointment, req, revalidatedSlot); validateErr != nil {
		uc.Log.Error("paymentUsecase.HandleAppointmentPayment proposed appointment validation failed",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String("appointmentId", req.AppointmentID),
			zap.Error(validateErr),
		)
		return nil, exceptions.BuildNewCustomError(
			validateErr,
			constvars.StatusBadRequest,
			"Appointment is not eligible for booking",
			validateErr.Error(),
		)
	}

	return uc.handleAppointmentBooking(ctx, req, precond, appointment, revalidatedSlot, slotID, requestID)
}

// handleAppointmentBooking executes the single booking flow: it reserves the
// slot as busy-tentative, moves the BFF-created proposed appointment to pending,
// creates a Xendit invoice, and returns the payment URL. On any failure the slot
// and appointment are rolled back to their pre-booking states.
func (uc *paymentUsecase) handleAppointmentBooking(
	ctx context.Context,
	req *requests.AppointmentPaymentRequest,
	precond *preconditionData,
	appointment *fhir_dto.Appointment,
	revalidatedSlot *fhir_dto.Slot,
	slotID string,
	requestID string,
) (*responses.AppointmentPaymentResponse, error) {
	revalidatedSlot.Status = fhir_dto.SlotStatusBusyTentative
	if _, updateErr := uc.SlotFhirClient.UpdateSlot(ctx, slotID, revalidatedSlot); updateErr != nil {
		uc.Log.Error("paymentUsecase.handleAppointmentBooking failed to update slot status",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String("slotId", slotID),
			zap.Error(updateErr),
		)
		return nil, exceptions.BuildNewCustomError(
			updateErr,
			constvars.StatusInternalServerError,
			"Failed to reserve slot. Please try again.",
			"slot update failed",
		)
	}

	// Move the proposed appointment to pending: accepted by the backend, awaiting payment
	appointment.Status = constvars.FhirAppointmentStatusPending
	if updateErr := uc.updateAppointment(ctx, appointment); updateErr != nil {
		uc.Log.Error("paymentUsecase.handleAppointmentBooking failed to update appointment status",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String("appointmentId", appointment.ID),
			zap.Error(updateErr),
		)
		uc.rollbackSlotToFree(ctx, slotID, revalidatedSlot, requestID)
		return nil, exceptions.BuildNewCustomError(
			updateErr,
			constvars.StatusInternalServerError,
			"Failed to update appointment. Please try again.",
			"appointment update failed",
		)
	}

	createInvoice := uc.CreateAppointmentInvoice
	if createInvoice == nil {
		createInvoice = uc.createXenditInvoiceForAppointment
	}
	url, xenditErr := createInvoice(ctx, req, precond)
	if xenditErr != nil {
		uc.Log.Error("paymentUsecase.handleAppointmentBooking failed to create Xendit invoice",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(xenditErr),
		)
		// Rollback slot to free and appointment to proposed to avoid reservation leak
		uc.rollbackSlotToFree(ctx, slotID, revalidatedSlot, requestID)
		appointment.Status = constvars.FhirAppointmentStatusProposed
		if rollbackErr := uc.updateAppointment(ctx, appointment); rollbackErr != nil {
			uc.Log.Error("paymentUsecase.handleAppointmentBooking failed to rollback appointment after Xendit failure",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.String("appointmentId", appointment.ID),
				zap.Error(rollbackErr),
			)
		}
		return nil, xenditErr
	}

	expiresAt := time.Now().Add(time.Duration(uc.InternalConfig.App.PaymentExpiredTimeInMinutes) * time.Minute)

	uc.Log.Info("paymentUsecase.HandleAppointmentPayment succeeded (reservation + pending appointment)",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String("slotId", slotID),
		zap.String("appointmentId", appointment.ID),
	)

	return &responses.AppointmentPaymentResponse{
		Status:        constvars.StatusCreated,
		Message:       constvars.AppointmentPaymentPendingMessage,
		AppointmentID: fmt.Sprintf("%s/%s", constvars.ResourceAppointment, appointment.ID),
		SlotID:        req.SlotID,
		PaymentURL:    url,
		ExpiresAt:     expiresAt.Format(time.RFC3339),
	}, nil
}

// rollbackSlotToFree restores a slot to the free status after a failed booking,
// best-effort: failures are logged but not returned.
func (uc *paymentUsecase) rollbackSlotToFree(ctx context.Context, slotID string, slot *fhir_dto.Slot, requestID string) {
	slot.Status = fhir_dto.SlotStatusFree
	if _, rollbackErr := uc.SlotFhirClient.UpdateSlot(ctx, slotID, slot); rollbackErr != nil {
		uc.Log.Error("paymentUsecase.rollbackSlotToFree failed to rollback slot",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String("slotId", slotID),
			zap.Error(rollbackErr),
		)
	}
}

func whitelistAccessByRoles(ctx context.Context, allowedRoles []string) bool {
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
	Slot              *fhir_dto.Slot
	PractitionerRole  *fhir_dto.PractitionerRole
	HealthcareService *fhir_dto.HealthcareService
	Practitioner      *fhir_dto.Practitioner
	Patient           *fhir_dto.Patient
	Invoice           *fhir_dto.Invoice
	Schedule          *fhir_dto.Schedule
}

// validatePatientIdentity fetches the patient and verifies the UID matches (or is superadmin).
func (uc *paymentUsecase) validatePatientIdentity(ctx context.Context, requestID, patientID, uid string, isSuperadmin bool) (*fhir_dto.Patient, error) {
	fetchedPatient, pErr := uc.PatientFhirClient.FindPatientByID(ctx, patientID)
	if pErr != nil {
		uc.Log.Error("paymentUsecase.ensurePreconditionsValid failed to fetch patient",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(pErr),
		)
		return nil, exceptions.BuildNewCustomError(
			pErr,
			constvars.StatusBadRequest,
			"Failed to fetch patient data",
			"precondition: patient fetch failed",
		)
	}
	match := false
	for _, identifier := range fetchedPatient.Identifier {
		if identifier.Value == uid {
			match = true
			break
		}
	}
	if !isSuperadmin && !match {
		err := errors.New("patient ID does not match with the current user")
		uc.Log.Error("paymentUsecase.ensurePreconditionsValid patient mismatch",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.BuildNewCustomError(
			err,
			constvars.StatusBadRequest,
			"Patient ID does not match with the current user",
			"precondition: patient mismatch",
		)
	}
	return fetchedPatient, nil
}

// validateSlotEligibility checks slot status, schedule ownership, and that the slot is in the future.
func (*paymentUsecase) validateSlotEligibility(slot *fhir_dto.Slot, schedules []fhir_dto.Schedule, schedulesErr error, slotID string) (*fhir_dto.Schedule, error) {
	if slot.Status != fhir_dto.SlotStatusFree {
		return nil, exceptions.BuildNewCustomError(
			nil,
			constvars.StatusConflict,
			constvars.SlotNoLongerAvailableMessage,
			fmt.Sprintf("slot %s has status %s, expected free", slotID, slot.Status),
		)
	}
	scheduleRef := slot.Schedule.Reference
	if schedulesErr != nil || len(schedules) == 0 {
		return nil, exceptions.BuildNewCustomError(
			schedulesErr,
			constvars.StatusBadRequest,
			"Failed to validate slot ownership",
			"failed to find schedule for practitioner role",
		)
	}
	var matchedSchedule *fhir_dto.Schedule
	for i := range schedules {
		if "Schedule/"+schedules[i].ID == scheduleRef {
			matchedSchedule = &schedules[i]
			break
		}
	}
	if matchedSchedule == nil {
		return nil, exceptions.BuildNewCustomError(
			nil,
			constvars.StatusBadRequest,
			"Slot does not belong to the specified practitioner role",
			fmt.Sprintf("slot schedule %s does not match practitioner role", scheduleRef),
		)
	}
	nowLocal := time.Now().In(slot.Start.Location())
	if slot.Start.Before(nowLocal) || slot.Start.Equal(nowLocal) {
		return nil, exceptions.BuildNewCustomError(
			nil,
			constvars.StatusBadRequest,
			constvars.SlotInPastMessage,
			fmt.Sprintf("slot start time %s is not in the future", slot.Start.Format(time.RFC3339)),
		)
	}
	return matchedSchedule, nil
}

// ensurePreconditionsValid fetches and validates all required resources
func (uc *paymentUsecase) ensurePreconditionsValid(
	ctx context.Context,
	req *requests.AppointmentPaymentRequest,
) (*preconditionData, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	uid, _ := ctx.Value(constvars.CONTEXT_UID).(string)
	roles, _ := ctx.Value(constvars.CONTEXT_FHIR_ROLE).([]string)
	isSuperadmin := slices.Contains(roles, constvars.KonsulinRoleSuperadmin)

	patientID := strings.TrimPrefix(req.PatientID, constvars.FHIRRefPrefixPatient)
	invoiceID := strings.TrimPrefix(req.InvoiceID, constvars.FHIRRefPrefixInvoice)

	fetchedPatient, err := uc.validatePatientIdentity(ctx, requestID, patientID, uid, isSuperadmin)
	if err != nil {
		return nil, err
	}

	res, err := uc.fetchCommonResources(ctx, req)
	if err != nil {
		return nil, err
	}

	// Invoice validations
	isInvoiceBelongsToPractitionerRole := slices.ContainsFunc(res.invoices[0].Participant, func(p fhir_dto.InvoiceParticipant) bool {
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
	if res.invoices[0].TotalNet == nil || res.invoices[0].TotalNet.Value <= 0 {
		return nil, exceptions.BuildNewCustomError(
			nil,
			constvars.StatusPreconditionFailed,
			"Invoice total amount must be greater than zero",
			fmt.Sprintf("invoice %s has invalid totalNet value", invoiceID),
		)
	}

	slotID := strings.TrimPrefix(req.SlotID, "Slot/")
	matchedSchedule, err := uc.validateSlotEligibility(res.slot, res.schedules, res.schedulesErr, slotID)
	if err != nil {
		return nil, err
	}

	return &preconditionData{
		Slot:              res.slot,
		PractitionerRole:  res.practitionerRole,
		HealthcareService: res.healthcareService,
		Practitioner:      res.practitioner,
		Patient:           fetchedPatient,
		Invoice:           &res.invoices[0],
		Schedule:          matchedSchedule,
	}, nil
}

// fetchHealthcareService fetches a HealthcareService by its logical ID using the shared
// FHIR HTTP client, which handles status code validation and OperationOutcome parsing.
func (uc *paymentUsecase) fetchHealthcareService(ctx context.Context, hsID string) (*fhir_dto.HealthcareService, error) {
	return fetchResourceByID[fhir_dto.HealthcareService](
		ctx,
		uc.FHIRClient,
		uc.InternalConfig.FHIR.BaseUrl,
		constvars.ResourceHealthcareService,
		hsID,
		"healthcare service",
	)
}

// The three shared test doubles below were relocated from
// appointment_payment_notification_test.go (deleted in T25B). They live here
// because appointment_booking_test.go, appointment_cross_role_test.go,
// appointment_payment_test.go and settlement_test.go still depend on them.

// mockSlotFhirClient mocks contracts.SlotFhirClient for payment tests.
type mockSlotFhirClient struct {
	slot            *fhir_dto.Slot
	err             error
	slotsBySchedule func(scheduleID string, params contracts.SlotSearchParams) ([]fhir_dto.Slot, error)
}

func (m *mockSlotFhirClient) FindSlotByID(_ context.Context, _ string) (*fhir_dto.Slot, error) {
	return m.slot, m.err
}

func (*mockSlotFhirClient) FindSlotByScheduleID(_ context.Context, _ string) ([]fhir_dto.Slot, error) {
	return nil, nil
}

func (*mockSlotFhirClient) FindSlotByScheduleAndTimeRange(_ context.Context, _ string, _, _ time.Time) ([]fhir_dto.Slot, error) {
	return nil, nil
}

func (*mockSlotFhirClient) FindSlotByScheduleIDAndStatus(_ context.Context, _, _ string) ([]fhir_dto.Slot, error) {
	return nil, nil
}

func (*mockSlotFhirClient) CreateSlot(_ context.Context, _ *fhir_dto.Slot) (*fhir_dto.Slot, error) {
	return nil, nil
}

func (*mockSlotFhirClient) UpdateSlot(_ context.Context, _ string, _ *fhir_dto.Slot) (*fhir_dto.Slot, error) {
	return nil, nil
}

func (m *mockSlotFhirClient) FindSlotsByScheduleWithQuery(_ context.Context, scheduleID string, params contracts.SlotSearchParams) ([]fhir_dto.Slot, error) {
	if m.slotsBySchedule != nil {
		return m.slotsBySchedule(scheduleID, params)
	}
	return nil, nil
}

func (*mockSlotFhirClient) PostTransactionBundle(_ context.Context, _ map[string]interface{}) (*fhir_dto.FHIRBundle, error) {
	return nil, nil
}

// mockSlotUsecase mocks contracts.SlotUsecaseIface for payment tests.
type mockSlotUsecase struct {
	lockErr          error
	lockPractitioner string
	lockStart        time.Time
	lockEnd          time.Time
}

func (*mockSlotUsecase) HandleSetUnavailabilityForMultiplePractitionerRoles(_ context.Context, _ contracts.SetUnavailabilityForMultiplePractitionerRolesInput) (*contracts.SetUnavailableOutcome, error) {
	return nil, nil
}

func (m *mockSlotUsecase) AcquireLocksForPractitionerDay(_ context.Context, practitionerID string, start, end time.Time, _ time.Duration) (func(context.Context), error) {
	m.lockPractitioner = practitionerID
	m.lockStart = start
	m.lockEnd = end
	if m.lockErr != nil {
		return func(_ context.Context) {}, m.lockErr
	}
	return func(_ context.Context) {}, nil
}

// mockAppointmentResourceClient stubs the raw FHIR HTTP operations used to
// fetch Appointment and HealthcareService resources during payment flows.
type mockAppointmentResourceClient struct {
	getResponses map[string][]byte
	gets         []string
}

func (m *mockAppointmentResourceClient) Do(_ context.Context, method, url string, _ io.Reader) ([]byte, error) {
	if method == http.MethodGet {
		m.gets = append(m.gets, url)
		if body := m.getResponses[url]; body != nil {
			return body, nil
		}
		return nil, &fhir_http_client.FHIRHTTPError{StatusCode: http.StatusNotFound, Err: errors.New("resource not found")}
	}
	return nil, fmt.Errorf("unexpected method %s for %s", method, url)
}
