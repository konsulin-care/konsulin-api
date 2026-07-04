package payments

import (
	"testing"
	"time"

	"konsulin-service/internal/pkg/fhir_dto"
)

func TestParseAppointmentExternalID(t *testing.T) {
	t.Run("parses valid external_id with all fields", func(t *testing.T) {
		externalID := "appointment:slot-123:pr-456:pat-789:inv-012:hs-999"
		fields, err := parseAppointmentExternalID(externalID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if fields.SlotID != "slot-123" {
			t.Errorf("expected SlotID 'slot-123', got '%s'", fields.SlotID)
		}
		if fields.PractitionerRoleID != "pr-456" {
			t.Errorf("expected PractitionerRoleID 'pr-456', got '%s'", fields.PractitionerRoleID)
		}
		if fields.PatientID != "pat-789" {
			t.Errorf("expected PatientID 'pat-789', got '%s'", fields.PatientID)
		}
		if fields.InvoiceID != "inv-012" {
			t.Errorf("expected InvoiceID 'inv-012', got '%s'", fields.InvoiceID)
		}
		if fields.HealthcareServiceID != "hs-999" {
			t.Errorf("expected HealthcareServiceID 'hs-999', got '%s'", fields.HealthcareServiceID)
		}
	})

	t.Run("errors on wrong number of parts", func(t *testing.T) {
		_, err := parseAppointmentExternalID("appointment:slot-123")
		if err == nil {
			t.Error("expected error for short external_id")
		}
	})

	t.Run("errors on wrong prefix", func(t *testing.T) {
		_, err := parseAppointmentExternalID("webhook:slot-123:pr-456:pat-789:inv-012:hs-999")
		if err == nil {
			t.Error("expected error for wrong prefix")
		}
	})

	t.Run("errors on empty slot ID", func(t *testing.T) {
		_, err := parseAppointmentExternalID("appointment::pr-456:pat-789:inv-012:hs-999")
		if err == nil {
			t.Error("expected error for empty slot ID")
		}
	})
}

func TestGetOverlappingNonFreeSlots(t *testing.T) {
	slotStart := time.Date(2026, 7, 4, 9, 0, 0, 0, time.UTC)
	slotEnd := time.Date(2026, 7, 4, 10, 0, 0, 0, time.UTC)

	t.Run("no overlap when no non-free slots exist", func(t *testing.T) {
		slots := []fhir_dto.Slot{}
		result := getOverlappingNonFreeSlots(slots, slotStart, slotEnd)
		if len(result) != 0 {
			t.Errorf("expected 0 overlaps, got %d", len(result))
		}
	})

	t.Run("detects overlap with busy-unavailable slot that ends after requested start", func(t *testing.T) {
		existing := fhir_dto.Slot{
			ID:     "slot-1",
			Status: fhir_dto.SlotStatusBusyUnavailable,
			Start:  time.Date(2026, 7, 4, 8, 30, 0, 0, time.UTC),
			End:    time.Date(2026, 7, 4, 9, 30, 0, 0, time.UTC),
		}
		result := getOverlappingNonFreeSlots([]fhir_dto.Slot{existing}, slotStart, slotEnd)
		if len(result) != 1 {
			t.Errorf("expected 1 overlap, got %d", len(result))
		}
	})

	t.Run("detects overlap with busy-tentative slot", func(t *testing.T) {
		existing := fhir_dto.Slot{
			ID:     "slot-2",
			Status: fhir_dto.SlotStatusBusyTentative,
			Start:  time.Date(2026, 7, 4, 9, 30, 0, 0, time.UTC),
			End:    time.Date(2026, 7, 4, 10, 30, 0, 0, time.UTC),
		}
		result := getOverlappingNonFreeSlots([]fhir_dto.Slot{existing}, slotStart, slotEnd)
		if len(result) != 1 {
			t.Errorf("expected 1 overlap, got %d", len(result))
		}
	})

	t.Run("no overlap when non-free slot ends before requested start", func(t *testing.T) {
		existing := fhir_dto.Slot{
			ID:     "slot-3",
			Status: fhir_dto.SlotStatusBusyUnavailable,
			Start:  time.Date(2026, 7, 4, 8, 0, 0, 0, time.UTC),
			End:    time.Date(2026, 7, 4, 9, 0, 0, 0, time.UTC),
		}
		result := getOverlappingNonFreeSlots([]fhir_dto.Slot{existing}, slotStart, slotEnd)
		if len(result) != 0 {
			t.Errorf("expected 0 overlaps, got %d", len(result))
		}
	})

	t.Run("no overlap when non-free slot starts after requested end", func(t *testing.T) {
		existing := fhir_dto.Slot{
			ID:     "slot-4",
			Status: fhir_dto.SlotStatusBusyTentative,
			Start:  time.Date(2026, 7, 4, 10, 0, 0, 0, time.UTC),
			End:    time.Date(2026, 7, 4, 11, 0, 0, 0, time.UTC),
		}
		result := getOverlappingNonFreeSlots([]fhir_dto.Slot{existing}, slotStart, slotEnd)
		if len(result) != 0 {
			t.Errorf("expected 0 overlaps, got %d", len(result))
		}
	})

	t.Run("ignores free slots even if overlapping", func(t *testing.T) {
		existing := fhir_dto.Slot{
			ID:     "slot-5",
			Status: fhir_dto.SlotStatusFree,
			Start:  time.Date(2026, 7, 4, 9, 0, 0, 0, time.UTC),
			End:    time.Date(2026, 7, 4, 10, 0, 0, 0, time.UTC),
		}
		result := getOverlappingNonFreeSlots([]fhir_dto.Slot{existing}, slotStart, slotEnd)
		if len(result) != 0 {
			t.Errorf("expected 0 overlaps for free slots, got %d", len(result))
		}
	})

	t.Run("detects exact same time as overlap", func(t *testing.T) {
		existing := fhir_dto.Slot{
			ID:     "slot-6",
			Status: fhir_dto.SlotStatusBusyUnavailable,
			Start:  time.Date(2026, 7, 4, 9, 0, 0, 0, time.UTC),
			End:    time.Date(2026, 7, 4, 10, 0, 0, 0, time.UTC),
		}
		result := getOverlappingNonFreeSlots([]fhir_dto.Slot{existing}, slotStart, slotEnd)
		if len(result) != 1 {
			t.Errorf("expected 1 overlap for exact same time range, got %d", len(result))
		}
	})

	t.Run("non-free slot fully contains requested slot", func(t *testing.T) {
		existing := fhir_dto.Slot{
			ID:     "slot-7",
			Status: fhir_dto.SlotStatusBusyTentative,
			Start:  time.Date(2026, 7, 4, 7, 0, 0, 0, time.UTC),
			End:    time.Date(2026, 7, 4, 11, 0, 0, 0, time.UTC),
		}
		result := getOverlappingNonFreeSlots([]fhir_dto.Slot{existing}, slotStart, slotEnd)
		if len(result) != 1 {
			t.Errorf("expected 1 overlap when existing slot fully contains requested, got %d", len(result))
		}
	})
}
