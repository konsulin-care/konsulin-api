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
	slotID := strings.TrimPrefix(req.SlotID, "Slot/")
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

	// Set slot status based on payment type
	if req.UseOnlinePayment {
		precond.Slot.Status = fhir_dto.SlotStatusBusyTentative
	} else {
		precond.Slot.Status = fhir_dto.SlotStatusBusyUnavailable
	}
	entries = append(entries, map[string]any{
		"request": map[string]any{
			"method": "PUT",
			"url":    constvars.ResourceSlot + "/" + slotID,
		},
		"resource": precond.Slot,
	})

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
				Actor:  fhir_dto.Reference{Reference: "Practitioner/" + precond.Practitioner.ID},
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

// buildSlotAdjustmentEntries generates bundle entries for slot adjustments across all practitioner roles
func (uc *paymentUsecase) buildSlotAdjustmentEntries(
	ctx context.Context,
	precond *preconditionData,
	allPractitionerRoles []fhir_dto.PractitionerRole,
) ([]map[string]any, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)

	var entries []map[string]any

	var scheduleConfig slot.ScheduleConfig
	if err := json.Unmarshal([]byte(precond.Schedule.Comment), &scheduleConfig); err != nil {
		return nil, exceptions.BuildNewCustomError(
			err,
			constvars.StatusInternalServerError,
			"Failed to parse schedule configuration",
			"failed to parse schedule config",
		)
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

		// silently using only the first schedule for the role
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
			dayStart := day
			dayEnd := dayStart.Add(24 * time.Hour)

			// Clip appointment window to the day segment
			segmentStart := precond.Slot.Start
			if segmentStart.Before(dayStart) {
				segmentStart = dayStart
			}
			segmentEnd := precond.Slot.End
			if segmentEnd.After(dayEnd) {
				segmentEnd = dayEnd
			}
			if !segmentEnd.After(segmentStart) {
				continue
			}

			params := contracts.SlotSearchParams{
				Start:  "lt" + dayEnd.Format(time.RFC3339),
				End:    "gt" + dayStart.Format(time.RFC3339),
				Status: "",
			}

			existingSlots, err := uc.SlotFhirClient.FindSlotsByScheduleWithQuery(ctx, schedule.ID, params)
			if err != nil {
				uc.Log.Warn("buildSlotAdjustmentEntries failed to fetch existing slots",
					zap.String(constvars.LoggingRequestIDKey, requestID),
					zap.String("roleId", role.ID),
					zap.Error(err),
				)
				continue
			}

			toDelete, toCreate, adjErr := slot.BuildSlotAdjustmentForAppointment(
				role,
				schedule,
				existingSlots,
				segmentStart,
				segmentEnd,
				precond.Slot.ID,
				scheduleConfig.SlotMinutes,
				scheduleConfig.BufferMinutes,
			)
			if adjErr != nil {
				uc.Log.Warn("buildSlotAdjustmentEntries failed to compute adjustments",
					zap.String(constvars.LoggingRequestIDKey, requestID),
					zap.String("roleId", role.ID),
					zap.Error(adjErr),
				)
				continue
			}

			for _, slotID := range toDelete {
				// avoid deleting the selected slot for appointment itself
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

// formatMoney formats Money to display string
func formatMoney(money *fhir_dto.Money) string {
	if money == nil {
		return "IDR 0"
	}
	return fmt.Sprintf("%s %.0f", money.Currency, money.Value)
}
