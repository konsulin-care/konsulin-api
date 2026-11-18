package payments

import (
	"fmt"
	"strings"

	"github.com/konsulin-care/konsulin-api/internal/pkg/constvars"
)

// appointmentIDCount is the number of identifiers an appointment opaque carries:
// slot, practitionerRole, patient, invoice, healthcareService and appointment.
const appointmentIDCount = 6

// appointmentOpaqueSeparator joins the six ids inside the opaque segment.
const appointmentOpaqueSeparator = "."

// maxAppointmentOpaqueLength bounds the opaque segment of an external ID.
const maxAppointmentOpaqueLength = 512

// BuildAppointmentOpaque joins the six appointment identifiers (slot,
// practitionerRole, patient, invoice, healthcareService, appointment) in order
// with '.' into a single opaque segment.
//
// Parameters:
//   - ids: the six identifiers, in the canonical order above.
//
// Returns the joined opaque string, or an error when the count is not six, an
// id contains '.', an id is empty, or the joined opaque exceeds 512 characters.
func BuildAppointmentOpaque(ids []string) (string, error) {
	if len(ids) != appointmentIDCount {
		return "", fmt.Errorf("appointment opaque: expected %d ids, got %d", appointmentIDCount, len(ids))
	}
	for i, id := range ids {
		if id == "" {
			return "", fmt.Errorf("appointment opaque: id %d is empty", i)
		}
		if strings.Contains(id, appointmentOpaqueSeparator) {
			return "", fmt.Errorf("appointment opaque: id %d contains %q", i, appointmentOpaqueSeparator)
		}
	}
	opaque := strings.Join(ids, appointmentOpaqueSeparator)
	if len(opaque) > maxAppointmentOpaqueLength {
		return "", fmt.Errorf("appointment opaque: length %d exceeds %d", len(opaque), maxAppointmentOpaqueLength)
	}
	return opaque, nil
}

// ParseAppointmentExternalID parses an external ID shaped as
// `<party>:appointment:<opaque>` where the opaque is the six appointment ids
// joined with '.' (see BuildAppointmentOpaque).
//
// Parameters:
//   - full: the full external ID to parse.
//   - ownParty: the party ID this adapter owns; the embedded party must match.
//
// Returns the six ids in canonical order (slot, practitionerRole, patient,
// invoice, healthcareService, appointment), or an error when the external ID is
// malformed, the party is foreign, the domain is not "appointment", or the
// opaque exceeds 512 characters.
func ParseAppointmentExternalID(full, ownParty string) ([]string, error) {
	parts := strings.Split(full, ":")
	if len(parts) != 3 {
		return nil, fmt.Errorf("appointment external id: expected <party>:appointment:<opaque>, got %d segments", len(parts))
	}
	party, domain, opaque := parts[0], parts[1], parts[2]
	if party != ownParty {
		return nil, fmt.Errorf("appointment external id: foreign party %q", party)
	}
	if domain != string(constvars.AppointmentPaymentService) {
		return nil, fmt.Errorf("appointment external id: expected domain %q, got %q", constvars.AppointmentPaymentService, domain)
	}
	if len(opaque) > maxAppointmentOpaqueLength {
		return nil, fmt.Errorf("appointment external id: opaque length %d exceeds %d", len(opaque), maxAppointmentOpaqueLength)
	}
	ids := strings.Split(opaque, appointmentOpaqueSeparator)
	if len(ids) != appointmentIDCount {
		return nil, fmt.Errorf("appointment external id: expected %d ids in opaque, got %d", appointmentIDCount, len(ids))
	}
	for i, id := range ids {
		if id == "" {
			return nil, fmt.Errorf("appointment external id: id %d is empty", i)
		}
	}
	return ids, nil
}
