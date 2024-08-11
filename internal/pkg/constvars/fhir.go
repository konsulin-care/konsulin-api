package constvars

const (
	ResourcePatient            = "Patient"
	ResourceClinician          = "Clinician"
	ResourceObservation        = "Observation"
	ResourceCondition          = "Condition"
	ResourceMedication         = "Medication"
	ResourceProcedure          = "Procedure"
	ResourceEncounter          = "Encounter"
	ResourceAllergyIntolerance = "AllergyIntolerance"
	ResourceImmunization       = "Immunization"
	ResourceAppointment        = "Appointment"
	ResourceCarePlan           = "CarePlan"
	ResourceDiagnosticReport   = "DiagnosticReport"
	ResourcePractitionerRole   = "PractitionerRole"
	ResourcePractitioner       = "Practitioner"
	ResourceSchedule           = "Schedule"
	ResourceSlot               = "Slot"
	ResourceOrganization       = "Organization"
	ResourceDevice             = "Device"
	ResourceLocation           = "Location"
	ResourceHealthcareService  = "HealthcareService"
)

const (
	FhirFetchResourceFilterName     = "%s?name=%s"
	FhirFetchResourceWithPagination = "%s?_offset=%d&_count=%d"
)

const (
	FhirSlotStatusBusy = "busy"
)

const (
	FhirAppointmentStatusBooked    = "booked"
	FhirAppointmentStatusProposed  = "proposed"
	FhirAppointmentStatusPending   = "pending"
	FhirAppointmentStatusFulfilled = "fulfilled"
	FhirAppointmentStatusArrived   = "arrived"
	FhirAppointmentStatusCancelled = "cancelled"
)

const (
	FhirParticipantStatusAccepted    = "accepted"
	FhirParticipantStatusDeclined    = "declined"
	FhirParticipantStatusTentative   = "tentative"
	FhirParticipantStatusNeedsAction = "needs-action"
)
