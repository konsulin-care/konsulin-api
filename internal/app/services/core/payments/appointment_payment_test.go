package payments

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/exceptions"
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
		externalID := "appointment:slot-123:pr-456:pat-789:inv-012:hs-999:appt-000"
		fields, err := parseAppointmentExternalID(externalID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertFieldEquals(t, fields.SlotID, "slot-123", "SlotID")
		assertFieldEquals(t, fields.PractitionerRoleID, "pr-456", "PractitionerRoleID")
		assertFieldEquals(t, fields.PatientID, "pat-789", "PatientID")
		assertFieldEquals(t, fields.InvoiceID, "inv-012", "InvoiceID")
		assertFieldEquals(t, fields.HealthcareServiceID, "hs-999", "HealthcareServiceID")
		assertFieldEquals(t, fields.AppointmentID, "appt-000", "AppointmentID")
	})

	t.Run("errors on wrong number of parts", func(t *testing.T) {
		_, err := parseAppointmentExternalID("appointment:slot-123")
		if err == nil {
			t.Error("expected error for short external_id")
		}
	})

	t.Run("errors on missing appointment ID", func(t *testing.T) {
		_, err := parseAppointmentExternalID("appointment:slot-123:pr-456:pat-789:inv-012:hs-999")
		if err == nil {
			t.Error("expected error for 6-part external_id missing appointment ID")
		}
	})

	t.Run("errors on wrong prefix", func(t *testing.T) {
		_, err := parseAppointmentExternalID("webhook:slot-123:pr-456:pat-789:inv-012:hs-999:appt-000")
		if err == nil {
			t.Error("expected error for wrong prefix")
		}
	})

	t.Run("errors on empty slot ID", func(t *testing.T) {
		_, err := parseAppointmentExternalID("appointment::pr-456:pat-789:inv-012:hs-999:appt-000")
		if err == nil {
			t.Error("expected error for empty slot ID")
		}
	})

	t.Run("errors on empty appointment ID", func(t *testing.T) {
		_, err := parseAppointmentExternalID("appointment:slot-123:pr-456:pat-789:inv-012:hs-999:")
		if err == nil {
			t.Error("expected error for empty appointment ID")
		}
	})
}

// mockPractitionerFhirClient mocks contracts.PractitionerFhirClient for payment tests.
type mockPractitionerFhirClient struct {
	practitioner *fhir_dto.Practitioner
	err          error
	byEmail      func(ctx context.Context, email string) ([]fhir_dto.Practitioner, error)
}

func (m *mockPractitionerFhirClient) FindPractitionerByID(_ context.Context, _ string) (*fhir_dto.Practitioner, error) {
	return m.practitioner, m.err
}
func (*mockPractitionerFhirClient) FindPractitionerByIdentifier(_ context.Context, _, _ string) ([]fhir_dto.Practitioner, error) {
	return nil, nil
}
func (m *mockPractitionerFhirClient) FindPractitionerByEmail(ctx context.Context, email string) ([]fhir_dto.Practitioner, error) {
	if m.byEmail != nil {
		return m.byEmail(ctx, email)
	}
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

// -------------------------------------------------------------------------
// Mock FHIR client implementations for fetchCommonResources tests
// -------------------------------------------------------------------------

// mockPractitionerRoleFhirClient mocks contracts.PractitionerRoleFhirClient.
type mockPractitionerRoleFhirClient struct {
	role *fhir_dto.PractitionerRole
	err  error
}

func (m *mockPractitionerRoleFhirClient) FindPractitionerRoleByID(_ context.Context, _ string) (*fhir_dto.PractitionerRole, error) {
	return m.role, m.err
}
func (*mockPractitionerRoleFhirClient) DeletePractitionerRoleByID(_ context.Context, _ string) error {
	return nil
}
func (*mockPractitionerRoleFhirClient) FindPractitionerRoleByOrganizationID(_ context.Context, _ string) ([]fhir_dto.PractitionerRole, error) {
	return nil, nil
}
func (*mockPractitionerRoleFhirClient) FindPractitionerRoleByCustomRequest(_ context.Context, _ *requests.FindAllCliniciansByClinicID) ([]fhir_dto.PractitionerRole, error) {
	return nil, nil
}
func (*mockPractitionerRoleFhirClient) FindPractitionerRoleByPractitionerID(_ context.Context, _ string) ([]fhir_dto.PractitionerRole, error) {
	return nil, nil
}
func (*mockPractitionerRoleFhirClient) FindPractitionerRoleByPractitionerIDAndOrganizationID(_ context.Context, _, _ string) ([]fhir_dto.PractitionerRole, error) {
	return nil, nil
}
func (*mockPractitionerRoleFhirClient) CreatePractitionerRoles(_ context.Context, _ interface{}) error {
	return nil
}
func (*mockPractitionerRoleFhirClient) CreatePractitionerRole(_ context.Context, _ *fhir_dto.PractitionerRole) (*fhir_dto.PractitionerRole, error) {
	return nil, nil
}
func (*mockPractitionerRoleFhirClient) UpdatePractitionerRole(_ context.Context, _ *fhir_dto.PractitionerRole) (*fhir_dto.PractitionerRole, error) {
	return nil, nil
}
func (*mockPractitionerRoleFhirClient) FindPractitionerRoleByPractitionerIDAndName(_ context.Context, _ *requests.FindClinicianByClinicianID) ([]fhir_dto.PractitionerRole, error) {
	return nil, nil
}
func (*mockPractitionerRoleFhirClient) Search(_ context.Context, _ contracts.PractitionerRoleSearchParams) ([]fhir_dto.PractitionerRole, error) {
	return nil, nil
}

// mockInvoiceFhirClient mocks contracts.InvoiceFhirClient.
type mockInvoiceFhirClient struct {
	invoices []fhir_dto.Invoice
	err      error
}

func (m *mockInvoiceFhirClient) Search(_ context.Context, _ contracts.InvoiceSearchParams) ([]fhir_dto.Invoice, error) {
	return m.invoices, m.err
}

// mockScheduleFhirClient mocks contracts.ScheduleFhirClient.
type mockScheduleFhirClient struct {
	schedules       []fhir_dto.Schedule
	schedulesByRole map[string][]fhir_dto.Schedule
	err             error
}

func (m *mockScheduleFhirClient) FindScheduleByPractitionerRoleID(_ context.Context, roleID string) ([]fhir_dto.Schedule, error) {
	if m.schedulesByRole != nil {
		if s, ok := m.schedulesByRole[roleID]; ok {
			return s, m.err
		}
		return nil, m.err
	}
	return m.schedules, m.err
}
func (*mockScheduleFhirClient) CreateSchedule(_ context.Context, _ *fhir_dto.Schedule) (*fhir_dto.Schedule, error) {
	return nil, nil
}
func (*mockScheduleFhirClient) FindScheduleByPractitionerID(_ context.Context, _ string) ([]fhir_dto.Schedule, error) {
	return nil, nil
}
func (*mockScheduleFhirClient) Search(_ context.Context, _ contracts.ScheduleSearchParams) ([]fhir_dto.Schedule, error) {
	return nil, nil
}

func TestResolveHealthcareServiceID(t *testing.T) {
	t.Run("uses request hsID when provided", func(t *testing.T) {
		req := &requests.AppointmentPaymentRequest{
			HealthcareServiceID: "HealthcareService/hs-123",
		}
		role := &fhir_dto.PractitionerRole{
			HealthcareService: []fhir_dto.Reference{
				{Reference: "HealthcareService/hs-999"},
			},
		}
		hsID := resolveHealthcareServiceID(req, role)
		assertFieldEquals(t, hsID, "hs-123", "hsID")
	})

	t.Run("falls back to role HealthcareService when req empty", func(t *testing.T) {
		req := &requests.AppointmentPaymentRequest{}
		role := &fhir_dto.PractitionerRole{
			HealthcareService: []fhir_dto.Reference{
				{Reference: "HealthcareService/hs-456"},
			},
		}
		hsID := resolveHealthcareServiceID(req, role)
		assertFieldEquals(t, hsID, "hs-456", "hsID")
	})

	t.Run("returns empty when both req and role are empty", func(t *testing.T) {
		req := &requests.AppointmentPaymentRequest{}
		role := &fhir_dto.PractitionerRole{}
		hsID := resolveHealthcareServiceID(req, role)
		assertFieldEquals(t, hsID, "", "hsID")
	})

	t.Run("returns empty when req empty and role is nil", func(t *testing.T) {
		req := &requests.AppointmentPaymentRequest{}
		hsID := resolveHealthcareServiceID(req, nil)
		assertFieldEquals(t, hsID, "", "hsID")
	})

	t.Run("returns empty when req empty and role has empty HealthcareService slice", func(t *testing.T) {
		req := &requests.AppointmentPaymentRequest{}
		role := &fhir_dto.PractitionerRole{
			HealthcareService: []fhir_dto.Reference{},
		}
		hsID := resolveHealthcareServiceID(req, role)
		assertFieldEquals(t, hsID, "", "hsID")
	})
}

func TestFetchCommonResources_EmptyHealthcareServiceID(t *testing.T) {
	t.Run("returns error when no HealthcareServiceID in request or PractitionerRole", func(t *testing.T) {
		uc := &paymentUsecase{
			SlotFhirClient: &mockSlotFhirClient{
				slot: &fhir_dto.Slot{ID: "slot-123"},
			},
			PractitionerRoleFhirClient: &mockPractitionerRoleFhirClient{
				role: &fhir_dto.PractitionerRole{
					ID:           "pr-456",
					Practitioner: fhir_dto.Reference{Reference: "Practitioner/prac-001"},
					// No HealthcareService — triggers the empty hsID guard
				},
			},
			InvoiceFhirClient: &mockInvoiceFhirClient{
				invoices: []fhir_dto.Invoice{{ID: "inv-789"}},
			},
			ScheduleFhirClient: &mockScheduleFhirClient{},
			// FHIRClient can be nil — early return prevents reaching fetchHealthcareService
			Log: zap.NewNop(),
		}

		req := &requests.AppointmentPaymentRequest{
			PatientID:          "Patient/pat-001",
			InvoiceID:          "Invoice/inv-789",
			PractitionerRoleID: "PractitionerRole/pr-456",
			SlotID:             "Slot/slot-123",
			// HealthcareServiceID intentionally omitted
		}

		res, err := uc.fetchCommonResources(context.Background(), req)
		if res != nil {
			t.Errorf("expected nil result, got %v", res)
		}
		if err == nil {
			t.Fatal("expected error but got nil")
		}

		var customErr *exceptions.CustomError
		if !errors.As(err, &customErr) {
			t.Fatalf("expected *exceptions.CustomError, got %T: %v", err, err)
		}
		if customErr.StatusCode != constvars.StatusBadRequest {
			t.Errorf("expected status 400, got %d", customErr.StatusCode)
		}
		if !strings.Contains(customErr.DevMessage, "healthcare service could not be resolved") {
			t.Errorf("expected dev message about unresolved healthcare service, got: %s", customErr.DevMessage)
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

// validProposedAppointmentRequest returns a booking request with an appointment
// reference that matches the given slot window.
func validProposedAppointmentRequest() *requests.AppointmentPaymentRequest {
	return &requests.AppointmentPaymentRequest{
		PatientID:          "Patient/pat-123",
		InvoiceID:          "Invoice/inv-456",
		PractitionerRoleID: "PractitionerRole/pr-789",
		SlotID:             "Slot/slot-012",
		AppointmentID:      "Appointment/appt-000",
	}
}

// proposedAppointmentFor builds a proposed Appointment that matches the given
// request and slot window, so tests can mutate a single field to force a failure.
func proposedAppointmentFor(req *requests.AppointmentPaymentRequest, slot *fhir_dto.Slot) *fhir_dto.Appointment {
	return &fhir_dto.Appointment{
		ResourceType: constvars.ResourceAppointment,
		ID:           "appt-000",
		Status:       constvars.FhirAppointmentStatusProposed,
		Start:        slot.Start,
		End:          slot.End,
		Slot: []fhir_dto.Reference{
			{Reference: req.SlotID},
		},
		Participant: []fhir_dto.AppointmentParticipant{
			{Actor: fhir_dto.Reference{Reference: req.PatientID}, Status: constvars.FhirParticipantStatusAccepted},
			{Actor: fhir_dto.Reference{Reference: req.PractitionerRoleID}, Status: constvars.FhirParticipantStatusAccepted},
		},
	}
}

func TestValidateProposedAppointment(t *testing.T) {
	req := validProposedAppointmentRequest()
	slot := &fhir_dto.Slot{
		ID:     "slot-012",
		Status: fhir_dto.SlotStatusFree,
		Start:  time.Date(2026, 7, 4, 9, 0, 0, 0, time.UTC),
		End:    time.Date(2026, 7, 4, 10, 0, 0, 0, time.UTC),
	}

	t.Run("accepts valid proposed appointment", func(t *testing.T) {
		err := validateProposedAppointment(proposedAppointmentFor(req, slot), req, slot)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("rejects nil appointment", func(t *testing.T) {
		err := validateProposedAppointment(nil, req, slot)
		assertErrorExpected(t, err, true)
	})

	t.Run("rejects non-proposed status", func(t *testing.T) {
		appt := proposedAppointmentFor(req, slot)
		appt.Status = constvars.FhirAppointmentStatusPending
		err := validateProposedAppointment(appt, req, slot)
		assertErrorExpected(t, err, true)
	})

	t.Run("rejects mismatched slot reference", func(t *testing.T) {
		appt := proposedAppointmentFor(req, slot)
		appt.Slot[0].Reference = "Slot/other-slot"
		err := validateProposedAppointment(appt, req, slot)
		assertErrorExpected(t, err, true)
	})

	t.Run("rejects appointment without slot reference", func(t *testing.T) {
		appt := proposedAppointmentFor(req, slot)
		appt.Slot = nil
		err := validateProposedAppointment(appt, req, slot)
		assertErrorExpected(t, err, true)
	})

	t.Run("rejects missing patient participant", func(t *testing.T) {
		appt := proposedAppointmentFor(req, slot)
		appt.Participant = []fhir_dto.AppointmentParticipant{
			{Actor: fhir_dto.Reference{Reference: req.PractitionerRoleID}, Status: constvars.FhirParticipantStatusAccepted},
		}
		err := validateProposedAppointment(appt, req, slot)
		assertErrorExpected(t, err, true)
	})

	t.Run("rejects missing practitioner role participant", func(t *testing.T) {
		appt := proposedAppointmentFor(req, slot)
		appt.Participant = []fhir_dto.AppointmentParticipant{
			{Actor: fhir_dto.Reference{Reference: req.PatientID}, Status: constvars.FhirParticipantStatusAccepted},
		}
		err := validateProposedAppointment(appt, req, slot)
		assertErrorExpected(t, err, true)
	})

	t.Run("rejects mismatched start time", func(t *testing.T) {
		appt := proposedAppointmentFor(req, slot)
		appt.Start = appt.Start.Add(15 * time.Minute)
		err := validateProposedAppointment(appt, req, slot)
		assertErrorExpected(t, err, true)
	})

	t.Run("rejects mismatched end time", func(t *testing.T) {
		appt := proposedAppointmentFor(req, slot)
		appt.End = appt.End.Add(15 * time.Minute)
		err := validateProposedAppointment(appt, req, slot)
		assertErrorExpected(t, err, true)
	})
}
