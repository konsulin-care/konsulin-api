package payments

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"konsulin-service/internal/pkg/fhir_dto"

	"go.uber.org/zap"
)

// assertFieldEquals is a test helper that reduces cognitive complexity by avoiding
// repetitive if/error branching for each field assertion.
func assertFieldEquals(t *testing.T, got, expected, fieldName string) {
	t.Helper()
	if got != expected {
		t.Errorf("expected %s '%s', got '%s'", fieldName, expected, got)
	}
}

// assertOverlapCount is a test helper for overlap count assertions.
func assertOverlapCount(t *testing.T, got, expected int, msg ...string) {
	t.Helper()
	if got != expected {
		detail := ""
		if len(msg) > 0 {
			detail = " " + msg[0]
		}
		t.Errorf("expected %d overlaps, got %d%s", expected, got, detail)
	}
}

// assertErrorExpected is a test helper that checks whether an error was expected.
func assertErrorExpected(t *testing.T, err error, wantErr bool) {
	t.Helper()
	if wantErr && err == nil {
		t.Error("expected error but got nil")
	}
	if !wantErr && err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestParseAppointmentExternalID(t *testing.T) {
	t.Run("parses valid external_id with all fields", func(t *testing.T) {
		externalID := "appointment:slot-123:pr-456:pat-789:inv-012:hs-999"
		fields, err := parseAppointmentExternalID(externalID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertFieldEquals(t, fields.SlotID, "slot-123", "SlotID")
		assertFieldEquals(t, fields.PractitionerRoleID, "pr-456", "PractitionerRoleID")
		assertFieldEquals(t, fields.PatientID, "pat-789", "PatientID")
		assertFieldEquals(t, fields.InvoiceID, "inv-012", "InvoiceID")
		assertFieldEquals(t, fields.HealthcareServiceID, "hs-999", "HealthcareServiceID")
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

// mockPractitionerFhirClient mocks contracts.PractitionerFhirClient for payment tests.
type mockPractitionerFhirClient struct {
	practitioner *fhir_dto.Practitioner
	err          error
}

func (m *mockPractitionerFhirClient) FindPractitionerByID(_ context.Context, _ string) (*fhir_dto.Practitioner, error) {
	return m.practitioner, m.err
}
func (*mockPractitionerFhirClient) FindPractitionerByIdentifier(_ context.Context, _, _ string) ([]fhir_dto.Practitioner, error) {
	return nil, nil
}
func (*mockPractitionerFhirClient) FindPractitionerByEmail(_ context.Context, _ string) ([]fhir_dto.Practitioner, error) {
	return nil, nil
}
func (*mockPractitionerFhirClient) FindPractitionerByPhone(_ context.Context, _ string) ([]fhir_dto.Practitioner, error) {
	return nil, nil
}
func (*mockPractitionerFhirClient) CreatePractitioner(_ context.Context, _ *fhir_dto.Practitioner) (*fhir_dto.Practitioner, error) {
	return nil, nil
}
func (*mockPractitionerFhirClient) UpdatePractitioner(_ context.Context, _ *fhir_dto.Practitioner) (*fhir_dto.Practitioner, error) {
	return nil, nil
}
func (*mockPractitionerFhirClient) PatchPractitioner(_ context.Context, _ *fhir_dto.Practitioner) (*fhir_dto.Practitioner, error) {
	return nil, nil
}

func TestFetchPractitioner(t *testing.T) {
	t.Run("returns practitioner when found", func(t *testing.T) {
		uc := &paymentUsecase{
			PractitionerFhirClient: &mockPractitionerFhirClient{
				practitioner: &fhir_dto.Practitioner{ID: "prac-123"},
			},
			Log: zap.NewNop(),
		}

		prac, err := uc.fetchPractitioner(context.Background(), "Practitioner/prac-123")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if prac == nil {
			t.Fatal("expected practitioner, got nil")
		}
		if prac.ID != "prac-123" {
			t.Errorf("expected prac-123, got %s", prac.ID)
		}
	})

	t.Run("returns error when client fails", func(t *testing.T) {
		uc := &paymentUsecase{
			PractitionerFhirClient: &mockPractitionerFhirClient{
				err: errors.New("connection failed"),
			},
			Log: zap.NewNop(),
		}

		prac, err := uc.fetchPractitioner(context.Background(), "Practitioner/prac-456")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if prac != nil {
			t.Errorf("expected nil practitioner on error, got %v", prac)
		}
		if !strings.Contains(err.Error(), "failed to fetch practitioner") {
			t.Errorf("expected error containing 'failed to fetch practitioner', got %v", err)
		}
	})
}

func TestGetOverlappingNonFreeSlots(t *testing.T) {
	slotStart := time.Date(2026, 7, 4, 9, 0, 0, 0, time.UTC)
	slotEnd := time.Date(2026, 7, 4, 10, 0, 0, 0, time.UTC)

	t.Run("no overlap when no non-free slots exist", func(t *testing.T) {
		var slots []fhir_dto.Slot
		result := getOverlappingNonFreeSlots(slots, slotStart, slotEnd)
		assertOverlapCount(t, len(result), 0)
	})

	t.Run("detects overlap with busy-unavailable slot that ends after requested start", func(t *testing.T) {
		existing := fhir_dto.Slot{
			ID:     "slot-1",
			Status: fhir_dto.SlotStatusBusyUnavailable,
			Start:  time.Date(2026, 7, 4, 8, 30, 0, 0, time.UTC),
			End:    time.Date(2026, 7, 4, 9, 30, 0, 0, time.UTC),
		}
		result := getOverlappingNonFreeSlots([]fhir_dto.Slot{existing}, slotStart, slotEnd)
		assertOverlapCount(t, len(result), 1)
	})

	t.Run("detects overlap with busy-tentative slot", func(t *testing.T) {
		existing := fhir_dto.Slot{
			ID:     "slot-2",
			Status: fhir_dto.SlotStatusBusyTentative,
			Start:  time.Date(2026, 7, 4, 9, 30, 0, 0, time.UTC),
			End:    time.Date(2026, 7, 4, 10, 30, 0, 0, time.UTC),
		}
		result := getOverlappingNonFreeSlots([]fhir_dto.Slot{existing}, slotStart, slotEnd)
		assertOverlapCount(t, len(result), 1)
	})

	t.Run("no overlap when non-free slot ends before requested start", func(t *testing.T) {
		existing := fhir_dto.Slot{
			ID:     "slot-3",
			Status: fhir_dto.SlotStatusBusyUnavailable,
			Start:  time.Date(2026, 7, 4, 8, 0, 0, 0, time.UTC),
			End:    time.Date(2026, 7, 4, 9, 0, 0, 0, time.UTC),
		}
		result := getOverlappingNonFreeSlots([]fhir_dto.Slot{existing}, slotStart, slotEnd)
		assertOverlapCount(t, len(result), 0)
	})

	t.Run("no overlap when non-free slot starts after requested end", func(t *testing.T) {
		existing := fhir_dto.Slot{
			ID:     "slot-4",
			Status: fhir_dto.SlotStatusBusyTentative,
			Start:  time.Date(2026, 7, 4, 10, 0, 0, 0, time.UTC),
			End:    time.Date(2026, 7, 4, 11, 0, 0, 0, time.UTC),
		}
		result := getOverlappingNonFreeSlots([]fhir_dto.Slot{existing}, slotStart, slotEnd)
		assertOverlapCount(t, len(result), 0)
	})

	t.Run("ignores free slots even if overlapping", func(t *testing.T) {
		existing := fhir_dto.Slot{
			ID:     "slot-5",
			Status: fhir_dto.SlotStatusFree,
			Start:  time.Date(2026, 7, 4, 9, 0, 0, 0, time.UTC),
			End:    time.Date(2026, 7, 4, 10, 0, 0, 0, time.UTC),
		}
		result := getOverlappingNonFreeSlots([]fhir_dto.Slot{existing}, slotStart, slotEnd)
		assertOverlapCount(t, len(result), 0, "free slots should be ignored")
	})

	t.Run("detects exact same time as overlap", func(t *testing.T) {
		existing := fhir_dto.Slot{
			ID:     "slot-6",
			Status: fhir_dto.SlotStatusBusyUnavailable,
			Start:  time.Date(2026, 7, 4, 9, 0, 0, 0, time.UTC),
			End:    time.Date(2026, 7, 4, 10, 0, 0, 0, time.UTC),
		}
		result := getOverlappingNonFreeSlots([]fhir_dto.Slot{existing}, slotStart, slotEnd)
		assertOverlapCount(t, len(result), 1, "exact same time should be overlap")
	})

	t.Run("non-free slot fully contains requested slot", func(t *testing.T) {
		existing := fhir_dto.Slot{
			ID:     "slot-7",
			Status: fhir_dto.SlotStatusBusyTentative,
			Start:  time.Date(2026, 7, 4, 7, 0, 0, 0, time.UTC),
			End:    time.Date(2026, 7, 4, 11, 0, 0, 0, time.UTC),
		}
		result := getOverlappingNonFreeSlots([]fhir_dto.Slot{existing}, slotStart, slotEnd)
		assertOverlapCount(t, len(result), 1, "full containment should be overlap")
	})
}
