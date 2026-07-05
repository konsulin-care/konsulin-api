package payments

import (
	"bytes"
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
	"konsulin-service/internal/app/services/core/webhook"
	bundleSvc "konsulin-service/internal/app/services/fhir_spark/bundle"
	"konsulin-service/internal/app/services/shared/jwtmanager"
	"konsulin-service/internal/app/services/shared/storage"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/dto/responses"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/fhir_dto"
	"konsulin-service/internal/pkg/fhir_http_client"

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
	PersonFhirClient           contracts.PersonFhirClient
	Storage                    *storage.ServiceRequestStorage
	PaymentGateway             contracts.PaymentGatewayService
	InvoiceFhirClient          contracts.InvoiceFhirClient
	PractitionerRoleFhirClient contracts.PractitionerRoleFhirClient
	SlotFhirClient             contracts.SlotFhirClient
	ScheduleFhirClient         contracts.ScheduleFhirClient
	BundleFhirClient           bundleSvc.BundleFhirClient
	SlotUsecase                contracts.SlotUsecaseIface
	XenditClient               *xendit.APIClient
	FHIRClient                 *fhir_http_client.FHIRHTTPClient
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
			PersonFhirClient:           personFhirClient,
			Storage:                    storageService,
			PaymentGateway:             paymentGateway,
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

func (uc *paymentUsecase) XenditInvoiceCallback(ctx context.Context, header *requests.XenditInvoiceCallbackHeader, body *requests.XenditInvoiceCallbackBody) error {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	uc.Log.Info("paymentUsecase.XenditInvoiceCallback called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String("invoice_id", body.ID),
		zap.String("external_id", body.ExternalID),
		zap.String("status", string(body.Status)),
	)

	// 1) Validate callback token
	if header.CallbackToken != uc.InternalConfig.Xendit.WebhookToken {
		uc.Log.Error("paymentUsecase.XenditInvoiceCallback invalid callback token",
			zap.String(constvars.LoggingRequestIDKey, requestID),
		)
		return exceptions.BuildNewCustomError(
			fmt.Errorf("invalid callback token"),
			constvars.StatusUnauthorized,
			"Invalid callback token",
			"x-callback-token mismatch",
		)
	}

	// 2) Early exit for PENDING status
	if body.Status == requests.XenditInvoiceStatusPending {
		uc.Log.Info("paymentUsecase.XenditInvoiceCallback pending status; ignoring",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String("invoice_id", body.ID),
		)
		return nil
	}

	// 3) Verify payment status with Xendit (only for PAID status)
	if body.Status == requests.XenditInvoiceStatusPaid {
		if err := uc.verifyPaymentStatus(ctx, body.ID, body.Status); err != nil {
			uc.Log.Error("paymentUsecase.XenditInvoiceCallback verification failed",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.String("invoice_id", body.ID),
				zap.Error(err),
			)
			// Return 500 to trigger Xendit retry
			return exceptions.BuildNewCustomError(
				err,
				constvars.StatusInternalServerError,
				"Payment verification failed",
				"failed to verify payment status with Xendit",
			)
		}
	}

	// 4) Parse external_id prefix and route to appropriate handler
	parts := strings.Split(body.ExternalID, ":")
	if len(parts) < 2 {
		uc.Log.Error("paymentUsecase.XenditInvoiceCallback invalid external_id format: missing prefix",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String("external_id", body.ExternalID),
		)
		return nil
	}
	prefix := parts[0]

	switch prefix {
	case string(constvars.AppointmentPaymentService):
		return uc.handleAppointmentPaymentNotification(ctx, body.ExternalID, body.Status)
	case string(constvars.WebhookPaymentService):
		return uc.handleWebhookPaymentNotification(ctx, body.ExternalID, body.Status)
	default:
		uc.Log.Error("paymentUsecase.XenditInvoiceCallback unknown payment service type",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String("prefix", prefix),
			zap.String("external_id", body.ExternalID),
		)
		return nil
	}
}

// handleWebhookPaymentNotification processes webhook service payment notifications
func (uc *paymentUsecase) handleWebhookPaymentNotification(ctx context.Context, externalID string, status requests.XenditInvoiceStatus) error {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)

	if status == requests.XenditInvoiceStatusExpired {
		uc.Log.Info("paymentUsecase.handleWebhookPaymentNotification expired status; ignoring",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String("external_id", externalID),
		)
		return nil
	}

	parts := strings.Split(externalID, ":")
	if len(parts) < 2 {
		uc.Log.Error("paymentUsecase.handleWebhookPaymentNotification invalid external_id format",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String("external_id", externalID),
		)
		return nil
	}
	partnerTrxID := parts[1]

	id, version, parseErr := parsePartnerTrxID(partnerTrxID)
	if parseErr != nil {
		uc.Log.Error("paymentUsecase.handleWebhookPaymentNotification invalid partner_trx_id format",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String("partner_trx_id", partnerTrxID),
			zap.Error(parseErr),
		)
		return nil
	}

	sr, err := uc.Storage.FhirClient.GetServiceRequestByIDAndVersion(ctx, id, version)
	if err != nil {
		uc.Log.Error("paymentUsecase.handleWebhookPaymentNotification failed fetching ServiceRequest",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil
	}

	note, err := extractNoteStorage(sr)
	if err != nil {
		uc.Log.Error("paymentUsecase.handleWebhookPaymentNotification failed parsing stored note",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil
	}

	uri, err := resolveInstantiatesURI(sr, note)
	if err != nil {
		uc.Log.Error("paymentUsecase.handleWebhookPaymentNotification failed resolving instantiatesUri",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil
	}
	if err := uc.callInstantiateURI(ctx, uri, note.RawBody); err != nil {
		uc.Log.Error("paymentUsecase.handleWebhookPaymentNotification failed calling instantiate URI",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil
	}

	uc.Log.Info("paymentUsecase.handleWebhookPaymentNotification completed successfully",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)
	return nil
}

// handleAppointmentPaymentNotification processes appointment payment notifications (Phase 2).
// PAID/SETTLED: update slot to busy-unavailable, create Appointment/PaymentNotice/PaymentReconciliation via FHIR bundle.
// EXPIRED: delete the slot from FHIR.
func (uc *paymentUsecase) handleAppointmentPaymentNotification(ctx context.Context, externalID string, status requests.XenditInvoiceStatus) error {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)

	fields, err := parseAppointmentExternalID(externalID)
	if err != nil {
		uc.Log.Error("paymentUsecase.handleAppointmentPaymentNotification failed parsing external_id",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String("external_id", externalID),
			zap.Error(err),
		)
		return exceptions.BuildNewCustomError(
			err,
			constvars.StatusBadRequest,
			"Invalid external_id format",
			"failed to parse appointment external_id",
		)
	}

	slot, err := uc.SlotFhirClient.FindSlotByID(ctx, fields.SlotID)
	if err != nil {
		// EXPIRED callbacks for already-deleted slots are idempotent.
		// Treat a missing slot as already-processed instead of returning an error.
		if status == requests.XenditInvoiceStatusExpired {
			uc.Log.Info("paymentUsecase.handleAppointmentPaymentNotification slot not found for EXPIRED; treating as already processed",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.String("slotId", fields.SlotID),
			)
			return nil
		}
		uc.Log.Error("paymentUsecase.handleAppointmentPaymentNotification failed fetching slot",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String("slotId", fields.SlotID),
			zap.Error(err),
		)
		return exceptions.BuildNewCustomError(
			err,
			constvars.StatusNotFound,
			"Slot not found",
			fmt.Sprintf("failed to fetch slot %s", fields.SlotID),
		)
	}

	// Acquire locks before mutation to prevent race conditions and TOCTOU
	release, lockErr := uc.SlotUsecase.AcquireLocksForSlot(ctx, slot, 30*time.Second)
	if lockErr != nil {
		uc.Log.Error("paymentUsecase.handleAppointmentPaymentNotification failed to acquire locks",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String("slotId", fields.SlotID),
			zap.Error(lockErr),
		)
		return exceptions.BuildNewCustomError(
			lockErr,
			constvars.StatusConflict,
			"Unable to acquire necessary locks for slot update. Please try again.",
			"lock acquisition failed",
		)
	}
	defer func() { release(context.Background()) }()

	// Re-fetch slot after acquiring locks to protect against TOCTOU
	revalidatedSlot, err := uc.SlotFhirClient.FindSlotByID(ctx, fields.SlotID)
	if err != nil {
		uc.Log.Error("paymentUsecase.handleAppointmentPaymentNotification failed re-fetching slot",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String("slotId", fields.SlotID),
			zap.Error(err),
		)
		return exceptions.BuildNewCustomError(
			err,
			constvars.StatusInternalServerError,
			"Failed to re-fetch slot",
			"failed to re-fetch slot",
		)
	}

	switch status {
	case requests.XenditInvoiceStatusPaid, requests.XenditInvoiceStatusSettled:
		return uc.handleAppointmentPaymentPaid(ctx, fields, revalidatedSlot)
	case requests.XenditInvoiceStatusExpired:
		return uc.handleAppointmentPaymentExpired(ctx, fields, revalidatedSlot)
	default:
		uc.Log.Info("paymentUsecase.handleAppointmentPaymentNotification unsupported status",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String("status", string(status)),
		)
		return exceptions.BuildNewCustomError(
			fmt.Errorf("unsupported status"),
			constvars.StatusBadRequest,
			"Unsupported status",
			fmt.Sprintf("unsupported status: %s", string(status)),
		)
	}
}

// handleAppointmentPaymentPaid handles a PAID/SETTLED callback for appointment payments.
// It updates the slot to busy-unavailable and creates Appointment/PaymentNotice/PaymentReconciliation.
func (uc *paymentUsecase) handleAppointmentPaymentPaid(
	ctx context.Context,
	fields appointmentExternalIDFields,
	slot *fhir_dto.Slot,
) error {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)

	// Idempotency check: if already busy-unavailable, skip
	if slot.Status == fhir_dto.SlotStatusBusyUnavailable {
		uc.Log.Info("paymentUsecase.handleAppointmentPaymentPaid slot already busy-unavailable",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String("slotId", fields.SlotID),
		)
		return nil
	}

	// Reconstruct the request from external_id fields for precondition data fetching
	req := &requests.AppointmentPaymentRequest{
		PatientID:           constvars.FHIRRefPrefixPatient + fields.PatientID,
		InvoiceID:           constvars.FHIRRefPrefixInvoice + fields.InvoiceID,
		PractitionerRoleID:  constvars.FHIRRefPrefixPractitionerRole + fields.PractitionerRoleID,
		SlotID:              "Slot/" + fields.SlotID,
		HealthcareServiceID: constvars.FHIRRefPrefixHealthcareService + fields.HealthcareServiceID,
	}

	precond, err := uc.fetchPreconditionData(ctx, req)
	if err != nil {
		uc.Log.Error("paymentUsecase.handleAppointmentPaymentPaid failed to fetch precondition data",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return err
	}

	// Fetch all practitioner roles for bundle building
	practitionerID := strings.TrimPrefix(precond.PractitionerRole.Practitioner.Reference, constvars.FHIRRefPrefixPractitioner)
	allPractitionerRoles, err := uc.PractitionerRoleFhirClient.Search(ctx, contracts.PractitionerRoleSearchParams{
		PractitionerID: practitionerID,
	})
	if err != nil {
		uc.Log.Error("paymentUsecase.handleAppointmentPaymentPaid failed to fetch practitioner roles",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String("practitionerId", practitionerID),
			zap.Error(err),
		)
		return exceptions.BuildNewCustomError(
			err,
			constvars.StatusInternalServerError,
			constvars.ErrDevFailedToFetchPractitionerRoles,
			constvars.ErrDevFailedToFetchPractitionerRoles,
		)
	}

	// Update slot status to busy-unavailable
	slot.Status = fhir_dto.SlotStatusBusyUnavailable

	// Build and execute FHIR bundle (Appointment, PaymentNotice, PaymentReconciliation, and Slot update)
	bundleEntries, appointmentID, _, buildErr := uc.buildAppointmentPaymentBundle(ctx, req, precond, allPractitionerRoles)
	if buildErr != nil {
		uc.Log.Error("paymentUsecase.handleAppointmentPaymentPaid failed to build bundle",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(buildErr),
		)
		return buildErr
	}

	// Include the slot status update in the same transaction bundle for atomicity
	slotUpdateEntry := map[string]any{
		"request": map[string]any{
			"method": "PUT",
			"url":    constvars.ResourceSlot + "/" + fields.SlotID,
		},
		"resource": slot,
	}
	bundleEntries = append(bundleEntries, slotUpdateEntry)

	bundle := map[string]any{
		"resourceType": "Bundle",
		"type":         "transaction",
		"entry":        bundleEntries,
	}
	if _, bundleErr := uc.BundleFhirClient.PostTransactionBundle(ctx, bundle); bundleErr != nil {
		uc.Log.Error("paymentUsecase.handleAppointmentPaymentPaid bundle execution failed",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(bundleErr),
		)
		return exceptions.BuildNewCustomError(
			bundleErr,
			constvars.StatusInternalServerError,
			"Failed to create appointment from payment. Please try again.",
			"FHIR bundle transaction failed",
		)
	}

	uc.Log.Info("paymentUsecase.handleAppointmentPaymentPaid appointment created",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String("slotId", fields.SlotID),
		zap.String("appointmentId", appointmentID),
	)

	// best effort webhook notification
	asyncCtx := context.WithoutCancel(ctx)
	go uc.notifyProviderAsync(asyncCtx, notifyProviderAsyncInput{
		patient:       precond.Patient,
		paymentDate:   time.Now().Format(time.RFC3339),
		timeSlotStart: precond.Slot.Start.Format(time.RFC3339),
		timeSlotEnd:   precond.Slot.End.Format(time.RFC3339),
		amount:        formatMoney(precond.Invoice.TotalNet),
		amountPaid:    formatMoney(precond.Invoice.TotalNet),
	})

	return nil
}

// handleAppointmentPaymentExpired handles an EXPIRED callback for appointment payments.
// It deletes the slot from FHIR to free up the time slot.
func (uc *paymentUsecase) handleAppointmentPaymentExpired(
	ctx context.Context,
	fields appointmentExternalIDFields,
	slot *fhir_dto.Slot,
) error {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)

	// Idempotency check: if slot is already free, skip
	if slot.Status == fhir_dto.SlotStatusFree {
		uc.Log.Info("paymentUsecase.handleAppointmentPaymentExpired slot already free",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String("slotId", fields.SlotID),
		)
		return nil
	}

	// Delete the slot via FHIR transaction bundle (no DeleteSlot method on client)
	bundle := map[string]any{
		"resourceType": "Bundle",
		"type":         "transaction",
		"entry": []map[string]any{
			{
				"request": map[string]any{
					"method": "DELETE",
					"url":    constvars.ResourceSlot + "/" + fields.SlotID,
				},
			},
		},
	}
	if _, err := uc.BundleFhirClient.PostTransactionBundle(ctx, bundle); err != nil {
		uc.Log.Error("paymentUsecase.handleAppointmentPaymentExpired failed to delete slot via bundle",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String("slotId", fields.SlotID),
			zap.Error(err),
		)
		return exceptions.BuildNewCustomError(
			err,
			constvars.StatusInternalServerError,
			"Failed to delete expired slot",
			"failed to delete slot via bundle",
		)
	}

	uc.Log.Info("paymentUsecase.handleAppointmentPaymentExpired slot deleted",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String("slotId", fields.SlotID),
	)
	return nil
}

// verifyPaymentStatus fetches the invoice from Xendit and verifies the status matches the webhook status
func (uc *paymentUsecase) verifyPaymentStatus(ctx context.Context, invoiceID string, expectedStatus requests.XenditInvoiceStatus) error {
	ctxTimeout, cancel := context.WithTimeout(ctx, time.Duration(uc.InternalConfig.App.PaymentGatewayRequestTimeoutInSeconds)*time.Second)
	defer cancel()

	if uc.XenditClient == nil {
		return fmt.Errorf("xendit client not initialized")
	}

	apiReq := uc.XenditClient.InvoiceApi.GetInvoiceById(ctxTimeout, invoiceID)
	inv, httpResp, xenditErr := apiReq.Execute()
	if xenditErr != nil {
		return uc.mapXenditError(ctx, xenditErr, httpResp)
	}

	if expectedStatus == requests.XenditInvoiceStatus(inv.GetStatus()) {
		return nil
	}

	invoiceStatus := requests.XenditInvoiceStatus(inv.GetStatus())

	if expectedStatus == requests.XenditInvoiceStatusPaid {
		// when expecting status PAID, the fetched invoice on xendit
		// can be either PAID or SETTLED and both should be valid
		// in this case behave as if the payment has been paid
		if invoiceStatus == requests.XenditInvoiceStatusPaid ||
			invoiceStatus == requests.XenditInvoiceStatusSettled {
			return nil
		}
	}

	return fmt.Errorf("invalid expected status")
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
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)

	// Log response body if available
	if httpResp != nil && httpResp.Body != nil {
		bodyBytes, readErr := io.ReadAll(httpResp.Body)
		if readErr == nil && len(bodyBytes) > 0 {
			uc.Log.Error("paymentUsecase.mapXenditError Xendit error response body",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.String("response_body", string(bodyBytes)),
				zap.Int("status_code", httpResp.StatusCode),
			)
		}
	}

	statusCode := constvars.StatusInternalServerError
	if httpResp != nil && httpResp.StatusCode > 0 {
		statusCode = httpResp.StatusCode
	} else if statusText := strings.TrimSpace(err.Status()); statusText != "" {
		if parsed, convErr := strconv.Atoi(statusText); convErr == nil {
			statusCode = parsed
		}
	}

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
		rawMsg = constvars.ErrClientCannotProcessRequest
	}

	devMsg := fmt.Sprintf("xendit error code=%s message=%s", err.ErrorCode(), rawMsg)
	wrappedErr := errors.New(devMsg)

	if err.ErrorCode() == "API_VALIDATION_ERROR" || statusCode == http.StatusBadRequest {
		return exceptions.BuildNewCustomError(wrappedErr, constvars.StatusBadRequest, rawMsg, devMsg)
	}

	return exceptions.BuildNewCustomError(wrappedErr, statusCode, rawMsg, devMsg)
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
	// Format: appointment:{slotID}:{practitionerRoleID}:{patientID}:{invoiceID}:{healthcareServiceID}
	externalID := fmt.Sprintf("%s:%s:%s:%s:%s:%s",
		constvars.AppointmentPaymentService,
		slotID,
		practitionerRoleID,
		patientID,
		invoiceID,
		healthcareServiceID,
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

	preferredNotificationChannel := []xinvoice.NotificationChannel{}

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

// appointmentExternalIDFields holds all identifiers parsed from the Xendit external_id.
type appointmentExternalIDFields struct {
	SlotID              string
	PractitionerRoleID  string
	PatientID           string
	InvoiceID           string
	HealthcareServiceID string
}

// parseAppointmentExternalID parses the new external_id format:
//
//	appointment:{slotID}:{practitionerRoleID}:{patientID}:{invoiceID}:{healthcareServiceID}
func parseAppointmentExternalID(externalID string) (appointmentExternalIDFields, error) {
	parts := strings.Split(externalID, ":")
	if len(parts) != 6 {
		return appointmentExternalIDFields{}, fmt.Errorf("invalid appointment external_id format: expected 6 colon-separated parts, got %d", len(parts))
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
	}
	if strings.TrimSpace(fields.SlotID) == "" {
		return appointmentExternalIDFields{}, fmt.Errorf("invalid appointment external_id format: slot ID is empty")
	}
	return fields, nil
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
	if !uc.whitelistAccessByRoles(ctx, []string{constvars.KonsulinRolePatient, constvars.KonsulinRoleSuperadmin}) {
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

	if req.UseOnlinePayment {
		return uc.handleOnlineAppointmentPayment(ctx, req, precond, revalidatedSlot, slotID, requestID)
	}

	return uc.handleOfflineAppointmentPayment(ctx, req, precond, allPractitionerRoles, revalidatedSlot, slotID, requestID)
}

// handleOnlineAppointmentPayment executes Phase 1 online payment: reserves the slot as
// busy-tentative and creates a Xendit invoice for the patient to complete payment.
func (uc *paymentUsecase) handleOnlineAppointmentPayment(
	ctx context.Context,
	req *requests.AppointmentPaymentRequest,
	precond *preconditionData,
	revalidatedSlot *fhir_dto.Slot,
	slotID string,
	requestID string,
) (*responses.AppointmentPaymentResponse, error) {
	revalidatedSlot.Status = fhir_dto.SlotStatusBusyTentative
	if _, updateErr := uc.SlotFhirClient.UpdateSlot(ctx, slotID, revalidatedSlot); updateErr != nil {
		uc.Log.Error("paymentUsecase.handleOnlineAppointmentPayment failed to update slot status",
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

	url, xenditErr := uc.createXenditInvoiceForAppointment(ctx, req, precond)
	if xenditErr != nil {
		uc.Log.Error("paymentUsecase.handleOnlineAppointmentPayment failed to create Xendit invoice",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(xenditErr),
		)
		// Rollback slot to free to avoid reservation leak
		revalidatedSlot.Status = fhir_dto.SlotStatusFree
		if _, rollbackErr := uc.SlotFhirClient.UpdateSlot(ctx, slotID, revalidatedSlot); rollbackErr != nil {
			uc.Log.Error("paymentUsecase.handleOnlineAppointmentPayment failed to rollback slot after Xendit failure",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.String("slotId", slotID),
				zap.Error(rollbackErr),
			)
		}
		return nil, xenditErr
	}

	expiresAt := time.Now().Add(time.Duration(uc.InternalConfig.App.PaymentExpiredTimeInMinutes) * time.Minute)

	uc.Log.Info("paymentUsecase.HandleAppointmentPayment succeeded (Phase 1 reservation)",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String("slotId", slotID),
	)

	return &responses.AppointmentPaymentResponse{
		Status:     constvars.StatusCreated,
		Message:    constvars.AppointmentPaymentPendingMessage,
		SlotID:     req.SlotID,
		PaymentURL: url,
		ExpiresAt:  expiresAt.Format(time.RFC3339),
	}, nil
}

// handleOfflineAppointmentPayment executes offline payment: updates the slot to
// busy-unavailable, builds and executes the FHIR bundle, then sends a best-effort
// webhook notification to the provider.
func (uc *paymentUsecase) handleOfflineAppointmentPayment(
	ctx context.Context,
	req *requests.AppointmentPaymentRequest,
	precond *preconditionData,
	allPractitionerRoles []fhir_dto.PractitionerRole,
	revalidatedSlot *fhir_dto.Slot,
	slotID string,
	requestID string,
) (*responses.AppointmentPaymentResponse, error) {
	// Set slot status for the bundle entry (will be included in the transaction bundle)
	revalidatedSlot.Status = fhir_dto.SlotStatusBusyUnavailable

	bundleEntries, appointmentID, paymentNoticeID, bundleErr := uc.buildAppointmentPaymentBundle(ctx, req, precond, allPractitionerRoles)
	if bundleErr != nil {
		uc.Log.Error("paymentUsecase.handleOfflineAppointmentPayment failed to build bundle",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(bundleErr),
		)
		return nil, bundleErr
	}

	// Include the slot status update in the same transaction bundle for atomicity
	slotUpdateEntry := map[string]any{
		"request": map[string]any{
			"method": "PUT",
			"url":    constvars.ResourceSlot + "/" + slotID,
		},
		"resource": revalidatedSlot,
	}
	bundleEntries = append(bundleEntries, slotUpdateEntry)

	bundle := map[string]any{
		"resourceType": "Bundle",
		"type":         "transaction",
		"entry":        bundleEntries,
	}
	if _, execErr := uc.BundleFhirClient.PostTransactionBundle(ctx, bundle); execErr != nil {
		uc.Log.Error("paymentUsecase.handleOfflineAppointmentPayment bundle execution failed",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(execErr),
		)
		return nil, exceptions.BuildNewCustomError(
			execErr,
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
		amountPaid:    "0",
	})

	uc.Log.Info("paymentUsecase.HandleAppointmentPayment succeeded (offline)",
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
func (_ *paymentUsecase) validateSlotEligibility(slot *fhir_dto.Slot, schedules []fhir_dto.Schedule, schedulesErr error, slotID, _ string) (*fhir_dto.Schedule, error) {
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
	matchedSchedule, err := uc.validateSlotEligibility(res.slot, res.schedules, res.schedulesErr, slotID, requestID)
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
	url := uc.InternalConfig.FHIR.BaseUrl + constvars.ResourceHealthcareService + "/" + hsID

	body, err := uc.FHIRClient.Do(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch healthcare service: %w", err)
	}

	var hs fhir_dto.HealthcareService
	if unmarshalErr := json.Unmarshal(body, &hs); unmarshalErr != nil {
		return nil, fmt.Errorf("failed to decode HealthcareService: %w", unmarshalErr)
	}
	return &hs, nil
}

// buildAppointmentPaymentBundle constructs the full transaction bundle entries and
// returns the entries alongside deterministically generated IDs for Appointment and PaymentNotice.
// duplicate helper definitions removed (see appointment_payment_helpers.go)
