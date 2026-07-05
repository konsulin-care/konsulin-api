package payments

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/app/services/core/slot"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/fhir_dto"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

// buildAppointmentPaymentBundle constructs all bundle entries for the appointment payment
func (uc *paymentUsecase) buildAppointmentPaymentBundle(
	ctx context.Context,
	req *requests.AppointmentPaymentRequest,
	precond *preconditionData,
	allPractitionerRoles []fhir_dto.PractitionerRole,
) ([]map[string]any, string, string, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)

	var entries []map[string]any

	paymentReconID := uuid.New().String()
	paymentNoticeID := uuid.New().String()
	conditionID := uuid.New().String()
	now := time.Now().UTC()
	nowStr := now.Format(time.RFC3339)

	paymentRecon := fhir_dto.PaymentReconciliation{
		ResourceType: constvars.ResourcePaymentReconciliation,
		Meta: fhir_dto.Meta{
			LastUpdated: now,
		},
		ID:            paymentReconID,
		Status:        fhir_dto.PaymentReconciliationStatusActive,
		Created:       nowStr,
		Outcome:       fhir_dto.PaymentReconciliationOutcomeComplete,
		PaymentDate:   now.Format(time.DateOnly),
		PaymentAmount: *precond.Invoice.TotalNet,
		Requestor: &fhir_dto.Reference{
			Reference: req.PractitionerRoleID,
		},
		PaymentIssuer: &fhir_dto.Reference{
			Reference: constvars.ResourceOrganization + "/" + constvars.KonsulinOrganizationResourceID,
		},
	}
	entries = append(entries, map[string]any{
		"request": map[string]any{
			"method": "PUT",
			"url":    constvars.ResourcePaymentReconciliation + "/" + paymentReconID,
		},
		"resource": paymentRecon,
	})

	paymentNotice := fhir_dto.PaymentNotice{
		ResourceType: constvars.ResourcePaymentNotice,
		ID:           paymentNoticeID,
		Meta: fhir_dto.Meta{
			LastUpdated: now,
		},
		Status:  fhir_dto.PaymentNoticeStatusActive,
		Created: nowStr,
		Request: &fhir_dto.Reference{
			Reference: req.InvoiceID,
		},
		Provider: &fhir_dto.Reference{
			Reference: req.PractitionerRoleID,
		},
		Payment: &fhir_dto.Reference{
			Reference: constvars.ResourcePaymentReconciliation + "/" + paymentReconID,
		},
		Recipient: &fhir_dto.Reference{
			Reference: constvars.ResourceOrganization + "/" + constvars.KonsulinOrganizationResourceID,
		},
		Amount: *precond.Invoice.TotalNet,
	}
	entries = append(entries, map[string]any{
		"request": map[string]any{
			"method": "PUT",
			"url":    constvars.ResourcePaymentNotice + "/" + paymentNoticeID,
		},
		"resource": paymentNotice,
	})

	if strings.TrimSpace(req.Condition) != "" {
		condition := fhir_dto.Condition{
			ResourceType: constvars.ResourceCondition,
			ID:           conditionID,
			Meta: fhir_dto.Meta{
				LastUpdated: now,
			},
			Subject: fhir_dto.Reference{
				Reference: req.PatientID,
			},
			Asserter: &fhir_dto.Reference{
				Reference: req.PatientID,
			},
			Evidence: []fhir_dto.ConditionEvidence{
				{
					Code: []fhir_dto.CodeableConcept{
						{
							Text: req.Condition,
						},
					},
				},
			},
		}
		entries = append(entries, map[string]any{
			"request": map[string]any{
				"method": "PUT",
				"url":    constvars.ResourceCondition + "/" + conditionID,
			},
			"resource": condition,
		})
	}

	// Slot status is updated directly by the caller (Phase 1: busy-tentative, Phase 2 PAID: busy-unavailable, offline: busy-unavailable)
	// No slot entry is added here to avoid conflicts with the direct update.

	appointmentID := uuid.New().String()
	appointmentTypeText := "Offline"
	if req.UseOnlinePayment {
		appointmentTypeText = "Online"
	}
	appointment := fhir_dto.Appointment{
		ResourceType: constvars.ResourceAppointment,
		ID:           appointmentID,
		Meta: fhir_dto.Meta{
			LastUpdated: now,
		},
		Status: constvars.FhirAppointmentStatusBooked,
		AppointmentType: fhir_dto.CodeableConcept{
			Text: appointmentTypeText,
		},
		Start:   precond.Slot.Start,
		End:     precond.Slot.End,
		Created: now,
		Slot: []fhir_dto.Reference{
			{Reference: req.SlotID},
		},
		Participant: []fhir_dto.AppointmentParticipant{
			{
				Actor:  fhir_dto.Reference{Reference: req.PatientID},
				Status: constvars.FhirParticipantStatusAccepted,
			},
			{
				Actor:  fhir_dto.Reference{Reference: constvars.FHIRRefPrefixPractitioner + precond.Practitioner.ID},
				Status: constvars.FhirParticipantStatusAccepted,
			},
			{
				Actor:  fhir_dto.Reference{Reference: req.PractitionerRoleID},
				Status: constvars.FhirParticipantStatusAccepted,
			},
		},
	}

	if strings.TrimSpace(req.Condition) != "" {
		appointment.ReasonReference = []fhir_dto.Reference{
			{Reference: constvars.ResourceCondition + "/" + conditionID},
		}
	}

	entries = append(entries, map[string]any{
		"request": map[string]any{
			"method": "PUT",
			"url":    constvars.ResourceAppointment + "/" + appointmentID,
		},
		"resource": appointment,
	})

	slotEntries, err := uc.buildSlotAdjustmentEntries(ctx, precond, allPractitionerRoles)
	if err != nil {
		uc.Log.Error("buildAppointmentPaymentBundle failed to build slot adjustments",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, "", "", err
	}
	entries = append(entries, slotEntries...)

	return entries, appointmentID, paymentNoticeID, nil
}

// DayAdjustmentConfig groups the intra-day parameters for slot adjustment computation.
type DayAdjustmentConfig struct {
	Day           time.Time
	DayEnd        time.Time
	SlotMinutes   int
	BufferMinutes int
}

// buildDayAdjustmentEntries computes slot adjustments for a single day within an appointment window.
func (uc *paymentUsecase) buildDayAdjustmentEntries(
	ctx context.Context,
	requestID string,
	precond *preconditionData,
	role fhir_dto.PractitionerRole,
	schedule fhir_dto.Schedule,
	config DayAdjustmentConfig,
) ([]map[string]any, error) {
	var entries []map[string]any

	// Clip appointment window to the day segment
	segmentStart := precond.Slot.Start
	if segmentStart.Before(config.Day) {
		segmentStart = config.Day
	}
	segmentEnd := precond.Slot.End
	if segmentEnd.After(config.DayEnd) {
		segmentEnd = config.DayEnd
	}
	if !segmentEnd.After(segmentStart) {
		return entries, nil
	}

	params := contracts.SlotSearchParams{
		Start:  "lt" + config.DayEnd.Format(time.RFC3339),
		End:    "gt" + config.Day.Format(time.RFC3339),
		Status: "",
	}

	existingSlots, err := uc.SlotFhirClient.FindSlotsByScheduleWithQuery(ctx, schedule.ID, params)
	if err != nil {
		uc.Log.Warn("buildDayAdjustmentEntries failed to fetch existing slots",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String("roleId", role.ID),
			zap.Error(err),
		)
		return nil, nil
	}

	toDelete, toCreate, adjErr := slot.BuildSlotAdjustmentForAppointment(
		role,
		schedule,
		existingSlots,
		segmentStart,
		segmentEnd,
		precond.Slot.ID,
		config.SlotMinutes,
		config.BufferMinutes,
	)
	if adjErr != nil {
		uc.Log.Warn("buildDayAdjustmentEntries failed to compute adjustments",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String("roleId", role.ID),
			zap.Error(adjErr),
		)
		return nil, nil
	}

	for _, slotID := range toDelete {
		if slotID == precond.Slot.ID {
			continue
		}
		entries = append(entries, map[string]any{
			"request": map[string]any{
				"method": "DELETE",
				"url":    constvars.ResourceSlot + "/" + slotID,
			},
		})
	}

	for _, newSlot := range toCreate {
		entries = append(entries, map[string]any{
			"request": map[string]any{
				"method": "POST",
				"url":    constvars.ResourceSlot,
			},
			"resource": map[string]any{
				"resourceType": constvars.ResourceSlot,
				"schedule": map[string]any{
					"reference": "Schedule/" + schedule.ID,
				},
				"status": string(newSlot.Status),
				"start":  newSlot.Start.Format(time.RFC3339),
				"end":    newSlot.End.Format(time.RFC3339),
				"meta": map[string]any{
					"tag": []map[string]any{{"code": slot.SlotTagSystemGenerated}},
				},
			},
		})
	}

	return entries, nil
}

// buildSlotAdjustmentEntries generates bundle entries for slot adjustments across all practitioner roles
func (uc *paymentUsecase) buildSlotAdjustmentEntries(
	ctx context.Context,
	precond *preconditionData,
	allPractitionerRoles []fhir_dto.PractitionerRole,
) ([]map[string]any, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)

	var entries []map[string]any

	slotMinutes, durErr := precond.HealthcareService.ServiceDurationMinutes()
	if durErr != nil {
		return nil, exceptions.BuildNewCustomError(
			durErr,
			constvars.StatusInternalServerError,
			"Failed to parse schedule duration",
			"failed to read serviceDuration extension from HealthcareService",
		)
	}
	bufferMinutes := 5
	if b, ok := precond.HealthcareService.ServiceBufferMinutes(); ok {
		bufferMinutes = b
	}

	for _, role := range allPractitionerRoles {
		schedules, err := uc.ScheduleFhirClient.FindScheduleByPractitionerRoleID(ctx, role.ID)
		if err != nil || len(schedules) == 0 {
			uc.Log.Warn("buildSlotAdjustmentEntries failed to find schedule for role",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.String("roleId", role.ID),
				zap.Error(err),
			)
			continue
		}

		schedule := schedules[0]

		loc, tzErr := role.GetPreferredTimezone()
		if tzErr != nil {
			uc.Log.Warn("buildSlotAdjustmentEntries failed to get timezone",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.String("roleId", role.ID),
				zap.Error(tzErr),
			)
			continue
		}

		slotStartLocal := precond.Slot.Start.In(loc)
		slotEndLocal := precond.Slot.End.In(loc)
		for day := time.Date(slotStartLocal.Year(), slotStartLocal.Month(), slotStartLocal.Day(), 0, 0, 0, 0, loc); !day.After(time.Date(slotEndLocal.Year(), slotEndLocal.Month(), slotEndLocal.Day(), 0, 0, 0, 0, loc)); day = day.Add(24 * time.Hour) {
			dayEnd := day.Add(24 * time.Hour)
			dayEntries, dayErr := uc.buildDayAdjustmentEntries(ctx, requestID, precond, role, schedule, DayAdjustmentConfig{
				Day: day, DayEnd: dayEnd, SlotMinutes: slotMinutes, BufferMinutes: bufferMinutes,
			})
			if dayErr != nil {
				return nil, dayErr
			}
			entries = append(entries, dayEntries...)
		}
	}

	return entries, nil
}

type notifyProviderAsyncInput struct {
	patient       *fhir_dto.Patient
	paymentDate   string
	timeSlotStart string
	timeSlotEnd   string
	amount        string
	amountPaid    string
}

// notifyProviderAsync sends webhook notification to provider (best effort)
func (uc *paymentUsecase) notifyProviderAsync(
	ctx context.Context,
	input notifyProviderAsyncInput,
) {
	payload := map[string]any{
		"patientName":   input.patient.FullName(),
		"paymentDate":   time.Now().Format(time.RFC3339),
		"timeSlotStart": input.timeSlotStart,
		"timeSlotEnd":   input.timeSlotEnd,
		"amount":        input.amount,
		"amountPaid":    input.amountPaid,
	}

	contact := make(map[string]string)
	if len(input.patient.Telecom) > 0 {
		for _, telecom := range input.patient.Telecom {
			if telecom.System == "phone" {
				contact["phone"] = telecom.Value
			} else if telecom.System == "email" {
				contact["email"] = telecom.Value
			}
		}
	}
	payload["contact"] = contact

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		uc.Log.Error("notifyProviderAsync failed to marshal payload",
			zap.Error(err),
		)
		return
	}

	webhookURL := strings.TrimRight(uc.InternalConfig.App.BaseUrl, "/") + "/hook/notify-provider"
	httpReq, err := http.NewRequestWithContext(ctx, constvars.MethodPost, webhookURL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		uc.Log.Error("notifyProviderAsync failed to create request",

			zap.Error(err),
		)
		return
	}

	httpReq.Header.Set(constvars.HeaderContentType, constvars.MIMEApplicationJSON)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		uc.Log.Error("notifyProviderAsync webhook call failed",
			zap.String("webhookURL", webhookURL),
			zap.Error(err),
		)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			uc.Log.Error("notifyProviderAsync failed to read response body",

				zap.Error(err),
			)
		}
		uc.Log.Warn("notifyProviderAsync webhook returned error status",

			zap.Int("statusCode", resp.StatusCode),
			zap.String("response", string(body)),
		)
		return
	}

	uc.Log.Info("notifyProviderAsync webhook called successfully")
}

// getOverlappingNonFreeSlots filters FHIR Slot query results for slots whose time range
// overlaps with [slotStart, slotEnd). Only non-free slots (busy-unavailable, busy-tentative)
// are considered as conflicts. Free slots are ignored since they are dynamically generated.
func getOverlappingNonFreeSlots(slots []fhir_dto.Slot, slotStart, slotEnd time.Time) []fhir_dto.Slot {
	var overlapping []fhir_dto.Slot
	for _, s := range slots {
		if s.Status != fhir_dto.SlotStatusBusyUnavailable && s.Status != fhir_dto.SlotStatusBusyTentative {
			continue
		}
		// Overlap condition: existing slot starts before requested ends AND existing slot ends after requested starts
		if s.Start.Before(slotEnd) && s.End.After(slotStart) {
			overlapping = append(overlapping, s)
		}
	}
	return overlapping
}

// fetchPreconditionData fetches all resources needed for appointment payment bundle building.
// Unlike ensurePreconditionsValid, it does NOT validate slot status (needed for callback where slot is busy-tentative).
func (uc *paymentUsecase) fetchPreconditionData(
	ctx context.Context,
	req *requests.AppointmentPaymentRequest,
) (*preconditionData, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)

	// Fetch patient separately since no identity validation is needed
	patientID := strings.TrimPrefix(req.PatientID, constvars.FHIRRefPrefixPatient)
	fetchedPatient, pErr := uc.PatientFhirClient.FindPatientByID(ctx, patientID)
	if pErr != nil {
		uc.Log.Error("fetchPreconditionData failed to fetch patient",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(pErr),
		)
		return nil, exceptions.BuildNewCustomError(
			pErr,
			constvars.StatusBadRequest,
			"Failed to fetch patient data. Please try again.",
			"precondition fetch failed",
		)
	}

	res, err := uc.fetchCommonResources(ctx, req)
	if err != nil {
		return nil, err
	}

	var schedule *fhir_dto.Schedule
	if res.schedulesErr == nil && len(res.schedules) > 0 {
		schedule = &res.schedules[0]
	}

	return &preconditionData{
		Slot:              res.slot,
		PractitionerRole:  res.practitionerRole,
		HealthcareService: res.healthcareService,
		Practitioner:      res.practitioner,
		Patient:           fetchedPatient,
		Invoice:           &res.invoices[0],
		Schedule:          schedule,
	}, nil
}

// formatMoney formats Money to display string
func formatMoney(money *fhir_dto.Money) string {
	if money == nil {
		return "IDR 0"
	}
	return fmt.Sprintf("%s %.0f", money.Currency, money.Value)
}

// fetchedResources holds all resources fetched by fetchCommonResources.
type fetchedResources struct {
	slot              *fhir_dto.Slot
	practitionerRole  *fhir_dto.PractitionerRole
	healthcareService *fhir_dto.HealthcareService
	practitioner      *fhir_dto.Practitioner
	invoices          []fhir_dto.Invoice
	schedules         []fhir_dto.Schedule
	schedulesErr      error
}

// resolveHealthcareServiceID derives the healthcare service ID from the request
// or falls back to the practitioner role's referenced service.
func resolveHealthcareServiceID(req *requests.AppointmentPaymentRequest, role *fhir_dto.PractitionerRole) string {
	hsID := strings.TrimPrefix(req.HealthcareServiceID, constvars.FHIRRefPrefixHealthcareService)
	if hsID == "" && role != nil && len(role.HealthcareService) > 0 {
		hsID = strings.TrimPrefix(role.HealthcareService[0].Reference, constvars.FHIRRefPrefixHealthcareService)
	}
	return hsID
}

// fetchPractitioner fetches a Practitioner by its FHIR reference (e.g. "Practitioner/abc-123").
func (uc *paymentUsecase) fetchPractitioner(ctx context.Context, practitionerRef string) (*fhir_dto.Practitioner, error) {
	practitionerID := strings.TrimPrefix(practitionerRef, constvars.FHIRRefPrefixPractitioner)
	practitioner, err := uc.PractitionerFhirClient.FindPractitionerByID(ctx, practitionerID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch practitioner: %w", err)
	}
	return practitioner, nil
}

// fetchCommonResources fetches all resources shared by fetchPreconditionData and
// ensurePreconditionsValid: slot, practitioner role, invoice, schedule, healthcare
// service, and practitioner. Patient fetch is handled by the caller.
func (uc *paymentUsecase) fetchCommonResources(
	ctx context.Context,
	req *requests.AppointmentPaymentRequest,
) (*fetchedResources, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)

	slotID := strings.TrimPrefix(req.SlotID, "Slot/")
	practitionerRoleID := strings.TrimPrefix(req.PractitionerRoleID, constvars.FHIRRefPrefixPractitionerRole)
	invoiceID := strings.TrimPrefix(req.InvoiceID, constvars.FHIRRefPrefixInvoice)

	g, gctx := errgroup.WithContext(ctx)
	var res fetchedResources

	g.Go(func() error {
		s, err := uc.SlotFhirClient.FindSlotByID(gctx, slotID)
		if err != nil {
			return &resourceFetchError{resource: "slot", err: err}
		}
		res.slot = s
		return nil
	})

	g.Go(func() error {
		pr, err := uc.PractitionerRoleFhirClient.FindPractitionerRoleByID(gctx, practitionerRoleID)
		if err != nil {
			return &resourceFetchError{resource: "practitionerRole", err: err}
		}
		res.practitionerRole = pr
		return nil
	})

	g.Go(func() error {
		inv, err := uc.InvoiceFhirClient.Search(gctx, contracts.InvoiceSearchParams{ID: invoiceID})
		if err != nil {
			return &resourceFetchError{resource: "invoice", err: err}
		}
		res.invoices = inv
		return nil
	})

	g.Go(func() error {
		sched, err := uc.ScheduleFhirClient.FindScheduleByPractitionerRoleID(gctx, practitionerRoleID)
		if err != nil {
			res.schedulesErr = err
			return nil //nolint:nilerr // intentionally storing error in schedulesErr for caller to handle
		}
		res.schedules = sched
		return nil
	})

	if err := g.Wait(); err != nil {
		resType := "unknown"
		if fe, ok := err.(*resourceFetchError); ok {
			resType = fe.resource
		}
		uc.Log.Error("fetchCommonResources failed to fetch resource",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String("resourceType", resType),
			zap.Error(err),
		)
		return nil, exceptions.BuildNewCustomError(
			fmt.Errorf("failed to fetch %s", resType),
			constvars.StatusBadRequest,
			"Failed to fetch required data. Please try again.",
			"precondition fetch failed",
		)
	}

	if len(res.invoices) != 1 {
		return nil, exceptions.BuildNewCustomError(
			nil,
			constvars.StatusNotFound,
			"Invoice not found or multiple invoices found",
			fmt.Sprintf("invoice %s not found or multiple invoices found", invoiceID),
		)
	}

	// Fetch HealthcareService — derive from PractitionerRole if not explicitly provided
	hsID := resolveHealthcareServiceID(req, res.practitionerRole)
	hs, hsErr := uc.fetchHealthcareService(ctx, hsID)
	if hsErr != nil {
		return nil, exceptions.BuildNewCustomError(
			hsErr,
			constvars.StatusInternalServerError,
			"Failed to fetch healthcare service",
			hsErr.Error(),
		)
	}
	res.healthcareService = hs

	// Fetch Practitioner
	practitioner, practErr := uc.fetchPractitioner(ctx, res.practitionerRole.Practitioner.Reference)
	if practErr != nil {
		return nil, exceptions.BuildNewCustomError(
			practErr,
			constvars.StatusInternalServerError,
			"failed to fetch practitioner",
			"failed to fetch practitioner",
		)
	}
	res.practitioner = practitioner

	return &res, nil
}
