package payments

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
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

	"go.uber.org/zap"
)

// GatewayDeps bundles the gateway federation dependencies the payment usecase
// carries for the appointment booking checkout: the outbound gateway clients
// (payments, webhooks, listings), the provider alias resolver bound to the
// configured aliases, and this adapter's own and automation party ids. It is
// passed to NewPaymentUsecase at bootstrap; tests that build paymentUsecase
// literals directly do not need it.
type GatewayDeps struct {
	// Payments creates gateway payments (seller-mode checkout).
	Payments contracts.PaymentClient
	// Webhooks invokes the gateway webhook endpoints.
	Webhooks contracts.WebhookClient
	// Listings reads and enlists marketplace listings.
	Listings contracts.ListingDirectory
	// ResolveProvider maps a provider alias to the providing party id.
	ResolveProvider func(alias string) (string, error)
	// OwnPartyID is APP_PARTY_ID — the party this adapter owns.
	OwnPartyID string
	// AutomationPartyID is APP_AUTOMATION_PARTY_ID.
	AutomationPartyID string
}

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
	FHIRClient                 appointmentResourceClient
	// Gateway holds the gateway federation deps (payments, webhooks, listings,
	// provider resolver, party ids) used by the default appointment checkout.
	Gateway GatewayDeps
	// CreateAppointmentInvoice is the injectable seam for the payment checkout
	// call made by handleAppointmentBooking; when nil it falls back to the
	// gateway checkout closure over CreateAppointmentCheckout. Tests stub it to
	// exercise the booking flow offline without a gateway or Xendit sandbox.
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
	invoiceFhirClient contracts.InvoiceFhirClient,
	practitionerRoleFhirClient contracts.PractitionerRoleFhirClient,
	slotFhirClient contracts.SlotFhirClient,
	scheduleFhirClient contracts.ScheduleFhirClient,
	bundleFhirClient bundleSvc.BundleFhirClient,
	slotUsecase contracts.SlotUsecaseIface,
	gatewayDeps GatewayDeps,
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
			FHIRClient:                 fhir_http_client.New(logger),
			Gateway:                    gatewayDeps,
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

// gatewayAppointmentCheckout is the default CreateAppointmentInvoice seam: it
// delegates to the T27A seller-mode gateway checkout (CreateAppointmentCheckout)
// over the six appointment ids, the precondition invoice and the
// APP_FRONTEND_DOMAIN return URLs, then persists the gateway payment id back
// onto the Invoice. It fails closed when the gateway payment client was not
// wired or the precondition invoice is missing.
func (uc *paymentUsecase) gatewayAppointmentCheckout(
	ctx context.Context,
	req *requests.AppointmentPaymentRequest,
	precond *preconditionData,
) (string, error) {
	if uc.Gateway.Payments == nil {
		return "", exceptions.ErrServerProcess(fmt.Errorf("gateway payment client not initialized"))
	}
	if precond == nil || precond.Invoice == nil {
		return "", errors.New("checkout invoice is required")
	}
	input := AppointmentCheckoutInput{
		SlotID:              strings.TrimPrefix(req.SlotID, constvars.ResourceSlot+"/"),
		PractitionerRoleID:  strings.TrimPrefix(req.PractitionerRoleID, constvars.FHIRRefPrefixPractitionerRole),
		PatientID:           strings.TrimPrefix(req.PatientID, constvars.FHIRRefPrefixPatient),
		InvoiceID:           strings.TrimPrefix(req.InvoiceID, constvars.FHIRRefPrefixInvoice),
		HealthcareServiceID: strings.TrimPrefix(req.HealthcareServiceID, constvars.FHIRRefPrefixHealthcareService),
		AppointmentID:       strings.TrimPrefix(req.AppointmentID, constvars.ResourceAppointment+"/"),
		Invoice:             precond.Invoice,
		FrontendDomain:      uc.InternalConfig.App.FrontendDomain,
	}
	return CreateAppointmentCheckout(ctx, uc.Gateway.Payments, uc.persistCheckoutInvoice, input)
}

// persistCheckoutInvoice persists the invoice carrying the gateway payment
// identifier through a full-replacement PUT, mirroring updateAppointment.
func (uc *paymentUsecase) persistCheckoutInvoice(ctx context.Context, invoice *fhir_dto.Invoice) error {
	invoiceID := strings.TrimPrefix(invoice.ID, constvars.ResourceInvoice+"/")
	url := uc.InternalConfig.FHIR.BaseUrl + constvars.ResourceInvoice + "/" + invoiceID

	payload, err := json.Marshal(invoice)
	if err != nil {
		return fmt.Errorf("failed to marshal invoice: %w", err)
	}

	if _, err := uc.FHIRClient.Do(ctx, http.MethodPut, url, bytes.NewReader(payload)); err != nil {
		return fmt.Errorf("failed to update invoice: %w", err)
	}
	return nil
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
// creates the gateway payment checkout, and returns the payment URL. On any
// failure the slot and appointment are rolled back to their pre-booking states.
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
		createInvoice = uc.gatewayAppointmentCheckout
	}
	url, checkoutErr := createInvoice(ctx, req, precond)
	if checkoutErr != nil {
		uc.Log.Error("paymentUsecase.handleAppointmentBooking failed to create payment checkout",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(checkoutErr),
		)
		// Rollback slot to free and appointment to proposed to avoid reservation leak
		uc.rollbackSlotToFree(ctx, slotID, revalidatedSlot, requestID)
		appointment.Status = constvars.FhirAppointmentStatusProposed
		if rollbackErr := uc.updateAppointment(ctx, appointment); rollbackErr != nil {
			uc.Log.Error("paymentUsecase.handleAppointmentBooking failed to rollback appointment after checkout failure",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.String("appointmentId", appointment.ID),
				zap.Error(rollbackErr),
			)
		}
		return nil, checkoutErr
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
