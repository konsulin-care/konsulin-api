package constvars

const (
	// Generic messages
	ResponseUnknown = "unknown"
	ResponseSuccess = "success"
	ResponseError   = "error"

	// User-related messages
	CreateUserSuccessMessage                 = "user created successfully"
	UpdateUserSuccessMessage                 = "user updated successfully"
	DeleteUserSuccessMessage                 = "user deleted successfully"
	GetProfileSuccessMessage                 = "get profile successfully"
	GetEducationLevelSuccessMessage          = "get education levels successfully"
	GetCitySuccessMessage                    = "get cities successfully"
	GetAppointmentSuccessMessage             = "get appointments successfully"
	GetGenderSuccessMessage                  = "get genders successfully"
	GetClinicsSuccessfully                   = "get clinics successfully"
	GetCliniciansSuccessfully                = "get clinicians successfully"
	GetClinicianSummarySuccessfully          = "get clinician summary successfully"
	VerifyWhatsAppOTPSuccessMessage          = "whatsapp otp successfully verified"
	PaymentRoutingCallbackSuccessfullyCalled = "payment routing callback successfully called"

	// Clinician-related messages
	CreateClinicianClinicsSuccessMessage              = "clinics successfully created for clinician"
	CreateClinicianPracticeAvailabilitySuccessMessage = "practice availability successfully updated for clinician"
	CreateClinicianPracticeInformationSuccessMessage  = "practice information successfully updated for clinician"
	DeleteClinicianClinicSuccessMessage               = "clinic successfully deleted for clinician"

	// Assessment messages
	GetAssessmentsSuccessMessage   = "get assessments successfully"
	CreateAssessmentSuccessMessage = "assessment successfully created"
	FindAssessmentSuccessMessage   = "assessment successfully found"
	UpdateAssessmentSuccessMessage = "assessment successfully updated"
	DeleteAssessmentSuccessMessage = "assessment successfully deleted"

	// Assessment Response messages
	CreateAssessmentResponseSuccessMessage = "assessment response successfully created"
	FindAssessmentResponseSuccessMessage   = "assessment response successfully found"
	UpdateAssessmentResponseSuccessMessage = "assessment response successfully updated"
	DeleteAssessmentResponseSuccessMessage = "assessment response successfully deleted"

	// Assessment Response messages
	CreateJournalSuccessMessage = "journal successfully created"
	FindJournalSuccessMessage   = "journal successfully found"
	UpdateJournalSuccessMessage = "journal successfully updated"
	DeleteJournalSuccessMessage = "journal successfully deleted"

	// Patient-related messages
	CreatePatientAppointmentSuccessMessage = "appoinment successfully created for patient"

	// Appointment payment messages
	AppointmentPaymentSuccessMessage = "Payment successful and appointment confirmed."
	OnlinePaymentNotImplementedMessage = "Online payment is not yet supported. Please use offline payment."
	SlotNoLongerAvailableMessage = "Selected slot is no longer available."
	InvalidReferenceFormatMessage = "Invalid reference format. Expected format: ResourceType/ID"
	SlotInPastMessage = "Slot start time must be in the future."

	// Auth messages
	WhatsAppOTPSuccessMessage    = "whatsapp OTP successfully sent to recipient number"
	LoginSuccessMessage          = "successfully login"
	LogoutSuccessMessage         = "successfully logout"
	ForgotPasswordSuccessMessage = "if an account with this email exists, you will receive a password reset link."
	ResetPasswordSuccessMessage  = "password already reset successfully"
	MagicLinkSuccessMessage      = "magic link successfully generated"
)
