package payments

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/fhir_dto"

	"go.uber.org/zap"
)

// Settlement statuses handled by HandleSettlement; anything else is
// acknowledged without writes.
const (
	settlementStatusPaid     = "PAID"
	settlementStatusExpired  = "EXPIRED"
	settlementStatusCanceled = "CANCELED"
)

// SettlementEvent is the payload of a gateway settlement notification
// (purpose settlement-notification). Status carries the settlement outcome
// (PAID, EXPIRED, CANCELED); Amount and Currency describe the settled money
// and are informational — this repository holds no pricing logic.
type SettlementEvent struct {
	PaymentID  string  `json:"payment_id"`
	ExternalID string  `json:"external_id"`
	Status     string  `json:"status"`
	Amount     float64 `json:"amount"`
	Currency   string  `json:"currency"`
}

// SettlementEventSink is the slice of the payment usecase the federation
// settlement-notification handler dispatches through. *paymentUsecase
// implements it via HandleSettlement; registerFederation type-asserts the
// contracts.PaymentUsecase it receives against this interface.
type SettlementEventSink interface {
	// HandleSettlement applies one settlement notification to the FHIR side
	// effects its external ID domain prescribes.
	HandleSettlement(ctx context.Context, event SettlementEvent) error
}

// DecodeSettlementEvent decodes the exact verified bytes of a settlement
// notification into a SettlementEvent.
//
// Parameters: body is the raw verified request body.
// Returns the event, or an error when the body is not valid JSON or the
// external_id/status fields are missing.
func DecodeSettlementEvent(body []byte) (SettlementEvent, error) {
	var event SettlementEvent
	if err := json.Unmarshal(body, &event); err != nil {
		return SettlementEvent{}, fmt.Errorf("settlement event: malformed body: %w", err)
	}
	if strings.TrimSpace(event.ExternalID) == "" {
		return SettlementEvent{}, fmt.Errorf("settlement event: external_id is required")
	}
	if strings.TrimSpace(event.Status) == "" {
		return SettlementEvent{}, fmt.Errorf("settlement event: status is required")
	}
	return event, nil
}

// settlementExternalDomain returns the domain segment of an external ID
// shaped as `<party>:<domain>:<rest>`.
//
// Parameters: externalID is the external ID to inspect.
// Returns the domain and true when at least two colon segments exist,
// otherwise ("", false).
func settlementExternalDomain(externalID string) (string, bool) {
	parts := strings.Split(externalID, ":")
	if len(parts) < 2 {
		return "", false
	}
	return parts[1], true
}

// settlementPartyID returns the own gateway party ID the appointment external
// ID must embed; an absent config yields the empty party so every foreign
// external ID is rejected by ParseAppointmentExternalID.
func (uc *paymentUsecase) settlementPartyID() string {
	if uc.InternalConfig == nil {
		return ""
	}
	return uc.InternalConfig.Gateway.PartyID
}

// HandleSettlement applies one gateway settlement notification. The external
// ID domain routes the event: the appointment domain drives FHIR side effects
// through T22's ParseAppointmentExternalID, any other domain (listing) is
// acknowledged without writes. Within the appointment domain, PAID sets the
// slot to busy, books the appointment and writes PaymentNotice plus
// PaymentReconciliation and the slot update in one transaction bundle, then
// fires the practitioner notification hook; a slot that is already busy is
// acknowledged without writes. EXPIRED and CANCELED delete slot, invoice and
// appointment in one transaction bundle. Other statuses are acknowledged
// without writes.
//
// Parameters: ctx carries the request-scoped values; event is the verified
// settlement notification payload.
// Returns nil when the event was applied or deliberately acknowledged, or an
// error when the appointment settlement could not be applied.
func (uc *paymentUsecase) HandleSettlement(ctx context.Context, event SettlementEvent) error {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	uc.Log.Info("paymentUsecase.HandleSettlement called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String("payment_id", event.PaymentID),
		zap.String("external_id", event.ExternalID),
		zap.String("status", event.Status),
	)

	domain, ok := settlementExternalDomain(event.ExternalID)
	if !ok {
		uc.Log.Warn("paymentUsecase.HandleSettlement external_id has no domain; acknowledged without writes",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String("external_id", event.ExternalID),
		)
		return nil
	}
	if domain != string(constvars.AppointmentPaymentService) {
		uc.Log.Info("paymentUsecase.HandleSettlement non-appointment domain; acknowledged without FHIR writes",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String("domain", domain),
		)
		return nil
	}

	ids, err := ParseAppointmentExternalID(event.ExternalID, uc.settlementPartyID())
	if err != nil {
		uc.Log.Error("paymentUsecase.HandleSettlement failed parsing appointment external_id",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String("external_id", event.ExternalID),
			zap.Error(err),
		)
		return exceptions.BuildNewCustomError(
			err,
			constvars.StatusBadRequest,
			"Invalid external_id format",
			"failed to parse appointment external_id",
		)
	}
	fields := appointmentExternalIDFields{
		SlotID:              ids[0],
		PractitionerRoleID:  ids[1],
		PatientID:           ids[2],
		InvoiceID:           ids[3],
		HealthcareServiceID: ids[4],
		AppointmentID:       ids[5],
	}

	switch strings.ToUpper(strings.TrimSpace(event.Status)) {
	case settlementStatusPaid:
		return uc.handleSettlementPaid(ctx, fields, requestID)
	case settlementStatusExpired, settlementStatusCanceled:
		// One transaction bundle deletes slot, invoice and appointment — the
		// expiry cleanup helper is shared with the legacy path (called, never edited).
		return uc.handleAppointmentPaymentExpired(ctx, fields, nil)
	default:
		uc.Log.Info("paymentUsecase.HandleSettlement status carries no FHIR side effect; acknowledged",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String("status", event.Status),
		)
		return nil
	}
}

// handleSettlementPaid applies a PAID settlement for the appointment domain:
// it sets the slot to busy (not busy-unavailable), moves the appointment to
// booked and writes PaymentNotice, PaymentReconciliation, the appointment and
// the slot update in one transaction bundle, then fires the practitioner
// notification hook best effort. A slot that is already busy is acknowledged
// without writes.
func (uc *paymentUsecase) handleSettlementPaid(ctx context.Context, fields appointmentExternalIDFields, requestID string) error {
	currentSlot, err := uc.SlotFhirClient.FindSlotByID(ctx, fields.SlotID)
	if err != nil || currentSlot == nil {
		uc.Log.Error("paymentUsecase.handleSettlementPaid failed fetching slot",
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
	if currentSlot.Status == fhir_dto.SlotStatusBusy {
		uc.Log.Info("paymentUsecase.handleSettlementPaid slot already busy; acknowledged without writes",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String("slotId", fields.SlotID),
		)
		return nil
	}

	req := &requests.AppointmentPaymentRequest{
		PatientID:           constvars.FHIRRefPrefixPatient + fields.PatientID,
		InvoiceID:           constvars.FHIRRefPrefixInvoice + fields.InvoiceID,
		PractitionerRoleID:  constvars.FHIRRefPrefixPractitionerRole + fields.PractitionerRoleID,
		SlotID:              constvars.ResourceSlot + "/" + fields.SlotID,
		HealthcareServiceID: constvars.FHIRRefPrefixHealthcareService + fields.HealthcareServiceID,
	}

	precond, precondErr := uc.fetchPreconditionData(ctx, req)
	if precondErr != nil {
		uc.Log.Error("paymentUsecase.handleSettlementPaid failed fetching precondition data",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(precondErr),
		)
		return precondErr
	}

	practitionerID := strings.TrimPrefix(precond.PractitionerRole.Practitioner.Reference, constvars.FHIRRefPrefixPractitioner)
	allPractitionerRoles, rolesErr := uc.PractitionerRoleFhirClient.Search(ctx, contracts.PractitionerRoleSearchParams{
		PractitionerID: practitionerID,
	})
	if rolesErr != nil {
		uc.Log.Error("paymentUsecase.handleSettlementPaid failed fetching practitioner roles",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String("practitionerId", practitionerID),
			zap.Error(rolesErr),
		)
		return exceptions.BuildNewCustomError(
			rolesErr,
			constvars.StatusInternalServerError,
			constvars.ErrDevFailedToFetchPractitionerRoles,
			constvars.ErrDevFailedToFetchPractitionerRoles,
		)
	}

	appointment, apptErr := uc.fetchAppointment(ctx, fields.AppointmentID)
	if apptErr != nil {
		uc.Log.Error("paymentUsecase.handleSettlementPaid failed fetching appointment",
			zap.Error(apptErr),
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String("appointmentId", fields.AppointmentID),
		)
		return exceptions.BuildNewCustomError(
			apptErr,
			constvars.StatusNotFound,
			"Appointment not found",
			fmt.Sprintf("failed to fetch appointment %s", fields.AppointmentID),
		)
	}
	appointment.Status = constvars.FhirAppointmentStatusBooked

	currentSlot.Status = fhir_dto.SlotStatusBusy

	bundleEntries, appointmentID, _, buildErr := uc.buildAppointmentPaymentBundle(ctx, req, precond, allPractitionerRoles, appointment)
	if buildErr != nil {
		uc.Log.Error("paymentUsecase.handleSettlementPaid failed building bundle",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(buildErr),
		)
		return buildErr
	}
	bundleEntries = append(bundleEntries, map[string]any{
		constvars.FhirFieldRequest: map[string]any{
			constvars.FhirFieldMethod: constvars.FhirBundleMethodPut,
			constvars.FhirFieldURL:    constvars.ResourceSlot + "/" + fields.SlotID,
		},
		constvars.FhirFieldResource: currentSlot,
	})

	bundle := map[string]any{
		constvars.FhirFieldResourceType: constvars.ResourceBundle,
		constvars.FhirBundleFieldType:   constvars.FhirBundleTypeTransaction,
		constvars.FhirFieldEntry:        bundleEntries,
	}
	if _, bundleErr := uc.BundleFhirClient.PostTransactionBundle(ctx, bundle); bundleErr != nil {
		uc.Log.Error("paymentUsecase.handleSettlementPaid bundle execution failed",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(bundleErr),
		)
		return exceptions.BuildNewCustomError(
			bundleErr,
			constvars.StatusInternalServerError,
			"Failed to settle appointment payment. Please try again.",
			"FHIR bundle transaction failed",
		)
	}

	uc.Log.Info("paymentUsecase.handleSettlementPaid slot booked and payment artifacts written",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String("slotId", fields.SlotID),
		zap.String("appointmentId", appointmentID),
	)

	// Practitioner notification hook: best effort, never fails the settlement.
	notification := notifyProviderAsyncInput{patient: precond.Patient}
	notification.paymentDate = time.Now().Format(time.RFC3339)
	notification.timeSlotStart = precond.Slot.Start.Format(time.RFC3339)
	notification.timeSlotEnd = precond.Slot.End.Format(time.RFC3339)
	notification.amount = formatMoney(precond.Invoice.TotalNet)
	notification.amountPaid = formatMoney(precond.Invoice.TotalNet)
	go uc.notifyProviderAsync(context.WithoutCancel(ctx), notification)

	return nil
}
