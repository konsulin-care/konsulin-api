package responses

import (
	"encoding/json"
	"testing"
)

func TestAppointmentPaymentResponse_Marshaling(t *testing.T) {
	t.Run("omits appointment and paymentNotice when empty with omitempty", func(t *testing.T) {
		resp := AppointmentPaymentResponse{
			Status:    201,
			Message:   "Payment invoice created. Please complete payment to confirm your appointment.",
			SlotID:    "Slot/abc-123",
			PaymentURL: "https://xendit.test/inv/xyz",
			ExpiresAt: "2026-07-04T11:00:00+07:00",
		}

		data, err := json.Marshal(resp)
		if err != nil {
			t.Fatalf("unexpected marshal error: %v", err)
		}

		var raw map[string]any
		if err := json.Unmarshal(data, &raw); err != nil {
			t.Fatalf("unexpected unmarshal error: %v", err)
		}

		// Should NOT have appointment key when empty
		if _, ok := raw["appointment"]; ok {
			t.Errorf("expected 'appointment' to be omitted when empty, but it was present: %v", raw["appointment"])
		}
		// Should NOT have paymentNotice key when empty
		if _, ok := raw["paymentNotice"]; ok {
			t.Errorf("expected 'paymentNotice' to be omitted when empty, but it was present: %v", raw["paymentNotice"])
		}
		// Should have expiresAt
		if v, ok := raw["expiresAt"]; !ok {
			t.Errorf("expected 'expiresAt' to be present, but was missing")
		} else if v != "2026-07-04T11:00:00+07:00" {
			t.Errorf("expected expiresAt '2026-07-04T11:00:00+07:00', got %v", v)
		}
		// Should have slot
		if v, ok := raw["slot"]; !ok || v != "Slot/abc-123" {
			t.Errorf("expected slot 'Slot/abc-123', got %v", v)
		}
	})

	t.Run("includes appointment and paymentNotice when set", func(t *testing.T) {
		resp := AppointmentPaymentResponse{
			Status:          201,
			Message:         "Payment successful and appointment confirmed.",
			AppointmentID:   "Appointment/appt-456",
			SlotID:          "Slot/abc-123",
			PaymentNoticeID: "PaymentNotice/pn-789",
			PaymentURL:      "",
			ExpiresAt:       "",
		}

		data, err := json.Marshal(resp)
		if err != nil {
			t.Fatalf("unexpected marshal error: %v", err)
		}

		var raw map[string]any
		if err := json.Unmarshal(data, &raw); err != nil {
			t.Fatalf("unexpected unmarshal error: %v", err)
		}

		if _, ok := raw["appointment"]; !ok {
			t.Errorf("expected 'appointment' to be present when set")
		}
		if _, ok := raw["paymentNotice"]; !ok {
			t.Errorf("expected 'paymentNotice' to be present when set")
		}
	})
}
