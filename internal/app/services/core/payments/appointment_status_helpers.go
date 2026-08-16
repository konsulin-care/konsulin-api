package payments

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/fhir_dto"
	"net/http"
	"time"
)

// appointmentResourceClient abstracts the raw FHIR HTTP operations the payment
// usecase needs to read and update Appointment resources, so tests can
// substitute a stub for the concrete fhir_http_client.FHIRHTTPClient.
type appointmentResourceClient interface {
	Do(ctx context.Context, method, url string, body io.Reader) ([]byte, error)
}

// fetchResourceByID fetches a FHIR resource of type T by its logical ID using the
// shared FHIR HTTP client, which handles status code validation and OperationOutcome
// parsing. The label is used in error messages (e.g. "appointment").
func fetchResourceByID[T any](
	ctx context.Context,
	client appointmentResourceClient,
	baseURL string,
	resourceType string,
	id string,
	label string,
) (*T, error) {
	url := baseURL + resourceType + "/" + id

	body, err := client.Do(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch %s: %w", label, err)
	}

	var resource T
	if unmarshalErr := json.Unmarshal(body, &resource); unmarshalErr != nil {
		return nil, fmt.Errorf("failed to decode %s: %w", label, unmarshalErr)
	}
	return &resource, nil
}

// fetchAppointment fetches a FHIR Appointment resource by its logical ID.
// A 404 from the FHIR server is surfaced as a not-found error so callers can
// distinguish "does not exist" from transient failures.
func (uc *paymentUsecase) fetchAppointment(ctx context.Context, appointmentID string) (*fhir_dto.Appointment, error) {
	return fetchResourceByID[fhir_dto.Appointment](
		ctx,
		uc.FHIRClient,
		uc.InternalConfig.FHIR.BaseUrl,
		constvars.ResourceAppointment,
		appointmentID,
		"appointment",
	)
}

// updateAppointment persists the full Appointment resource via PUT, following
// FHIR full-replacement semantics: every field present in the resource is
// written back, and any field omitted is cleared.
func (uc *paymentUsecase) updateAppointment(ctx context.Context, appointment *fhir_dto.Appointment) error {
	url := uc.InternalConfig.FHIR.BaseUrl + constvars.ResourceAppointment + "/" + appointment.ID

	payload, err := json.Marshal(appointment)
	if err != nil {
		return fmt.Errorf("failed to marshal appointment: %w", err)
	}

	if _, err := uc.FHIRClient.Do(ctx, http.MethodPut, url, bytes.NewReader(payload)); err != nil {
		return fmt.Errorf("failed to update appointment: %w", err)
	}
	return nil
}

// validateProposedAppointment rejects a BFF-created Appointment unless it is
// still in the proposed state and its references (slot, patient, practitioner
// role) and time window all match the booking request and the target slot.
func validateProposedAppointment(appointment *fhir_dto.Appointment, req *requests.AppointmentPaymentRequest, slot *fhir_dto.Slot) error {
	if appointment == nil {
		return fmt.Errorf("appointment is required")
	}
	if appointment.Status != constvars.FhirAppointmentStatusProposed {
		return fmt.Errorf("appointment status must be %q, got %q", constvars.FhirAppointmentStatusProposed, appointment.Status)
	}
	if len(appointment.Slot) == 0 || appointment.Slot[0].Reference != req.SlotID {
		return fmt.Errorf("appointment slot reference %v does not match requested slot %s", appointment.Slot, req.SlotID)
	}
	if !appointmentHasParticipant(appointment, req.PatientID) {
		return fmt.Errorf("appointment is missing patient participant %s", req.PatientID)
	}
	if !appointmentHasParticipant(appointment, req.PractitionerRoleID) {
		return fmt.Errorf("appointment is missing practitioner role participant %s", req.PractitionerRoleID)
	}
	if !appointment.Start.Equal(slot.Start) || !appointment.End.Equal(slot.End) {
		return fmt.Errorf("appointment window [%s, %s] does not match slot window [%s, %s]",
			appointment.Start.Format(time.RFC3339), appointment.End.Format(time.RFC3339),
			slot.Start.Format(time.RFC3339), slot.End.Format(time.RFC3339))
	}
	return nil
}

// appointmentHasParticipant reports whether any participant actor reference
// equals the given FHIR reference.
func appointmentHasParticipant(appointment *fhir_dto.Appointment, reference string) bool {
	for _, participant := range appointment.Participant {
		if participant.Actor.Reference == reference {
			return true
		}
	}
	return false
}
