package requests

import (
	"testing"
)

// baseAppointmentPaymentRequest returns an AppointmentPaymentRequest with
// fixed common fields and the given healthcare service ID.
func baseAppointmentPaymentRequest(hsID string) *AppointmentPaymentRequest {
	return &AppointmentPaymentRequest{
		PatientID:           "Patient/pat-123",
		InvoiceID:           "Invoice/inv-456",
		PractitionerRoleID:  "PractitionerRole/pr-789",
		SlotID:              "Slot/slot-012",
		HealthcareServiceID: hsID,
	}
}

func TestAppointmentPaymentRequest_Validate_HealthcareServiceID(t *testing.T) {
	tests := []struct {
		name    string
		req     *AppointmentPaymentRequest
		wantErr bool
	}{
		{
			name:    "valid with healthcare service id",
			req:     baseAppointmentPaymentRequest("HealthcareService/hs-999"),
			wantErr: false,
		},
		{
			name:    "empty healthcare service id is valid (optional field)",
			req:     baseAppointmentPaymentRequest(""),
			wantErr: false,
		},
		{
			name:    "invalid healthcare service id format returns error",
			req:     baseAppointmentPaymentRequest("Invalid/format/wrong"),
			wantErr: true,
		},
		{
			name:    "wrong resource type prefix returns error",
			req:     baseAppointmentPaymentRequest("Slot/hs-999"),
			wantErr: true,
		},
		{
			name:    "healthcare service id with empty id part returns error",
			req:     baseAppointmentPaymentRequest("HealthcareService/"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if tt.wantErr && err == nil {
				t.Errorf("expected error but got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}
