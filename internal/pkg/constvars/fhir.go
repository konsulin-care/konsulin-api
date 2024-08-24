package constvars

type ResourceType string

const (
	ResourcePatient               = "Patient"
	ResourceClinician             = "Clinician"
	ResourceObservation           = "Observation"
	ResourceCondition             = "Condition"
	ResourceMedication            = "Medication"
	ResourceProcedure             = "Procedure"
	ResourceEncounter             = "Encounter"
	ResourceAllergyIntolerance    = "AllergyIntolerance"
	ResourceImmunization          = "Immunization"
	ResourceAppointment           = "Appointment"
	ResourceCarePlan              = "CarePlan"
	ResourceDiagnosticReport      = "DiagnosticReport"
	ResourcePractitionerRole      = "PractitionerRole"
	ResourcePractitioner          = "Practitioner"
	ResourceSchedule              = "Schedule"
	ResourceSlot                  = "Slot"
	ResourceOrganization          = "Organization"
	ResourceQuestionnaire         = "Questionnaire"
	ResourceQuestionnaireResponse = "QuestionnaireResponse"
	ResourceDevice                = "Device"
	ResourceLocation              = "Location"
	ResourceHealthcareService     = "HealthcareService"
)

const (
	FhirFetchResourceTypeAll   = "all"
	FhirFetchResourceTypePaged = "paged"
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

const (
	FhirCurrencyPrefixIndonesia = "Rp"
)
