package responses

import (
	"encoding/json"
	"testing"
)

// marshalAndUnmarshal is a test helper that marshals a response and unmarshals to a map.
func marshalAndUnmarshal(t *testing.T, resp AppointmentPaymentResponse) map[string]any {
	t.Helper()
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("unexpected marshal error: %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unexpected unmarshal error: %v", err)
	}
	return raw
}

// assertFieldOmitted verifies a key is absent from the marshalled JSON.
func assertFieldOmitted(t *testing.T, raw map[string]any, key string) {
	t.Helper()
	if _, ok := raw[key]; ok {
		t.Errorf("expected '%s' to be omitted when empty, but it was present: %v", key, raw[key])
	}
}

// assertFieldPresent verifies a key exists in the marshalled JSON.
func assertFieldPresent(t *testing.T, raw map[string]any, key string) {
	t.Helper()
	if _, ok := raw[key]; !ok {
		t.Errorf("expected '%s' to be present, but was missing", key)
	}
}

// assertFieldValue verifies a key exists and has the expected value.
func assertFieldValue(t *testing.T, raw map[string]any, key, expected string) {
	t.Helper()
	v, ok := raw[key]
	if !ok {
		t.Errorf("expected '%s' to be present, but was missing", key)
	} else if v != expected {
		t.Errorf("expected %s '%s', got %v", key, expected, v)
	}
}

func TestAppointmentPaymentResponse_Marshaling(t *testing.T) {
	t.Run("omits appointment and paymentNotice when empty with omitempty", func(t *testing.T) {
		resp := AppointmentPaymentResponse{
			Status:     201,
			Message:    "Payment invoice created. Please complete payment to confirm your appointment.",
			SlotID:     "Slot/abc-123",
			PaymentURL: "https://xendit.test/inv/xyz",
			ExpiresAt:  "2026-07-04T11:00:00+07:00",
		}

	raw := marshalAndUnmarshal(t, resp)

	assertFieldOmitted(t, raw, "appointment")
	assertFieldOmitted(t, raw, "paymentNotice")
	assertFieldValue(t, raw, "expiresAt", "2026-07-04T11:00:00+07:00")
	assertFieldValue(t, raw, "slot", "Slot/abc-123")
}

func TestAppointmentPaymentResponse_IncludesSetFields(t *testing.T) {
	resp := AppointmentPaymentResponse{
		Status:          201,
		Message:         "Payment successful and appointment confirmed.",
		AppointmentID:   "Appointment/appt-456",
		SlotID:          "Slot/abc-123",
		PaymentNoticeID: "PaymentNotice/pn-789",
		PaymentURL:      "",
		ExpiresAt:       "",
	}

	raw := marshalAndUnmarshal(t, resp)

	assertFieldPresent(t, raw, "appointment")
	assertFieldPresent(t, raw, "paymentNotice")
}
