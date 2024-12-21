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
	ResourceChargeItemDefinition  = "ChargeItemDefinition"
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
	FhirMonetaryComponentStatusBase          = "base"
	FhirMonetaryComponentStatusSurcharge     = "surcharge"
	FhirMonetaryComponentStatusDiscount      = "discount"
	FhirMonetaryComponentStatusTax           = "tax"
	FhirMonetaryComponentStatusInformational = "informational"
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
	FhirChargeItemDefinitionStatusActive  = "active"
	FhirChargeItemDefinitionStatusDraft   = "draft"
	FhirChargeItemDefinitionStatusRetired = "retired"
	FhirChargeItemDefinitionStatusUnknown = "unknown"
)

const (
	FhirCurrencyPrefixIndonesia = "Rp"
)

const (
	DEFAULT_CLINICIAN_PRACTICE_START_TIME_PARAMS = "00:00:00"
	DEFAULT_CLINICIAN_PRACTICE_END_TIME_PARAMS   = "23:59:59"

	DEFAULT_CLINICIAN_DESIRED_DAYS_PARAMS = "sun,mon,tue,wed,thu,fri,sat"
)
