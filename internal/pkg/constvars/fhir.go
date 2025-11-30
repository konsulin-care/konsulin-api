package constvars

type ResourceType string

const (
	FhirSystemSupertokenIdentifier = "https://login.konsulin.care/userid"
)

const (
	ResourcePatient                  = "Patient"
	ResourceGroup                    = "Group"
	ResourceClinician                = "Clinician"
	ResourceObservation              = "Observation"
	ResourceCondition                = "Condition"
	ResourceMedication               = "Medication"
	ResourceProcedure                = "Procedure"
	ResourceEncounter                = "Encounter"
	ResourceAllergyIntolerance       = "AllergyIntolerance"
	ResourceImmunization             = "Immunization"
	ResourceAppointment              = "Appointment"
	ResourceCarePlan                 = "CarePlan"
	ResourceChargeItemDefinition     = "ChargeItemDefinition"
	ResourceDiagnosticReport         = "DiagnosticReport"
	ResourcePractitionerRole         = "PractitionerRole"
	ResourcePractitioner             = "Practitioner"
	ResourceSchedule                 = "Schedule"
	ResourceSlot                     = "Slot"
	ResourceOrganization             = "Organization"
	ResourceQuestionnaire            = "Questionnaire"
	ResourceQuestionnaireResponse    = "QuestionnaireResponse"
	ResourceResearchStudy            = "ResearchStudy"
	ResourceDevice                   = "Device"
	ResourceLocation                 = "Location"
	ResourceHealthcareService        = "HealthcareService"
	ResourceServiceRequest           = "ServiceRequest"
	ResourceInvoice                  = "Invoice"
	ResourcePaymentReconciliation    = "PaymentReconciliation"
	ResourcePaymentNotice            = "PaymentNotice"
	ResourceMedicationRequest        = "MedicationRequest"
	ResourceMedicationAdministration = "MedicationAdministration"
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
	FhirObservationStatusRegistered     = "registered"
	FhirObservationStatusPreliminary    = "preliminary"
	FhirObservationStatusFinal          = "final"
	FhirObservationStatusAmended        = "amended"
	FhirObservationStatusCorrected      = "corrected"
	FhirObservationStatusCancelled      = "cancelled"
	FhirObservationStatusEnteredInError = "entered-in-error"
	FhirObservationStatusUnknown        = "unknown"
	FhirObservationJournalTitle         = "Journal Title"
	FhirObservationJournalBody          = "Journal Body"
)

const (
	FhirCurrencyPrefixIndonesia = "Rp"
)

const (
	DEFAULT_CLINICIAN_PRACTICE_START_TIME_PARAMS = "00:00:00"
	DEFAULT_CLINICIAN_PRACTICE_END_TIME_PARAMS   = "23:59:59"

	DEFAULT_CLINICIAN_DESIRED_DAYS_PARAMS = "sun,mon,tue,wed,thu,fri,sat"
)

const (
	FhirSupertokenSystemIdentifier      = "https://login.konsulin.care/userid"
	KonsulinOmnichannelSystemIdentifier = "https://login.konsulin.care/chatwoot-id"
)
