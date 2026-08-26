package payments

import (
	"context"
	"encoding/json"
	"io"
	"testing"

	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/fhir_dto"
)

// recordingFetchClient records the URL passed to Do and returns a minimal
// encoded Appointment so fetchResourceByID can unmarshal it.
type recordingFetchClient struct {
	url string
}

func (c *recordingFetchClient) Do(_ context.Context, _, url string, _ io.Reader) ([]byte, error) {
	c.url = url
	return json.Marshal(&fhir_dto.Appointment{ResourceType: constvars.ResourceAppointment, ID: "appt-000"})
}

// TestFetchResourceByID_StripsResourceTypePrefix guards against double-prefixed
// fetch URLs: HandleAppointmentPayment passes req.AppointmentID in
// reference-prefixed form ("Appointment/appt-000") while the paid-callback
// passes a bare ID, and both must resolve to a single-prefix URL.
func TestFetchResourceByID_StripsResourceTypePrefix(t *testing.T) {
	tests := []struct {
		name string
		id   string
	}{
		{name: "reference-prefixed id (booking request form)", id: "Appointment/appt-000"},
		{name: "bare id (paid-callback form)", id: "appt-000"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &recordingFetchClient{}

			appt, err := fetchResourceByID[fhir_dto.Appointment](
				context.Background(),
				client,
				"http://fhir.test/",
				constvars.ResourceAppointment,
				tt.id,
				"appointment",
			)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if appt == nil || appt.ID != "appt-000" {
				t.Fatalf("expected fetched appointment id appt-000, got %+v", appt)
			}
			if client.url != "http://fhir.test/Appointment/appt-000" {
				t.Errorf("expected single-prefix fetch URL, got %q", client.url)
			}
		})
	}
}
