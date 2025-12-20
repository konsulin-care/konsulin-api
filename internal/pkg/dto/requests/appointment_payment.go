package requests

import (
	"errors"
	"fmt"
	"konsulin-service/internal/pkg/constvars"
	"strings"
)

type AppointmentPaymentRequest struct {
	PatientID          string `json:"patientId"`
	InvoiceID          string `json:"invoiceId"`
	UseOnlinePayment   bool   `json:"useOnlinePayment"`
	PractitionerRoleID string `json:"practitionerRoleId"`
	SlotID             string `json:"slotId"`
	Condition          string `json:"condition"`
}

// Validate checks required fields and reference formats.
func (r *AppointmentPaymentRequest) Validate() error {
	if strings.TrimSpace(r.PatientID) == "" {
		return errors.New("patientId is required")
	}
	if strings.TrimSpace(r.InvoiceID) == "" {
		return errors.New("invoiceId is required")
	}
	if strings.TrimSpace(r.PractitionerRoleID) == "" {
		return errors.New("practitionerRoleId is required")
	}
	if strings.TrimSpace(r.SlotID) == "" {
		return errors.New("slotId is required")
	}

	if !isValidReference(r.PatientID, constvars.ResourcePatient) {
		return fmt.Errorf("patientId must follow format: %s/ID", constvars.ResourcePatient)
	}
	if !isValidReference(r.InvoiceID, constvars.ResourceInvoice) {
		return fmt.Errorf("invoiceId must follow format: %s/ID", constvars.ResourceInvoice)
	}
	if !isValidReference(r.PractitionerRoleID, constvars.ResourcePractitionerRole) {
		return fmt.Errorf("practitionerRoleId must follow format: %s/ID", constvars.ResourcePractitionerRole)
	}
	if !isValidReference(r.SlotID, constvars.ResourceSlot) {
		return fmt.Errorf("slotId must follow format: %s/ID", constvars.ResourceSlot)
	}

	return nil
}

// isValidReference checks if a reference follows the "ResourceType/ID" format
func isValidReference(reference string, expectedResourceType string) bool {
	parts := strings.Split(reference, "/")
	if len(parts) != 2 {
		return false
	}
	if parts[0] != expectedResourceType {
		return false
	}
	if strings.TrimSpace(parts[1]) == "" {
		return false
	}
	return true
}
