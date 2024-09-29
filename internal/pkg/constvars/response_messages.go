package constvars

const (
	// Generic messages
	ResponseUnknown = "unknown"
	ResponseSuccess = "success"
	ResponseError   = "error"

	// User-related messages
	CreateUserSuccessMessage        = "user created successfully"
	UpdateUserSuccessMessage        = "user updated successfully"
	DeleteUserSuccessMessage        = "user deleted successfully"
	GetProfileSuccessMessage        = "get profile successfully"
	GetEducationLevelSuccessMessage = "get education levels successfully"
	GetGenderSuccessMessage         = "get genders successfully"
	GetClinicsSuccessfully          = "get clinics successfully"
	GetCliniciansSuccessfully       = "get clinicians successfully"
	GetClinicianSummarySuccessfully = "get clinician summary successfully"

	// Clinician-related messages
	CreateClinicianClinicsSuccessMessage              = "clinics successfully created for clinician"
	CreateClinicianPracticeAvailabilitySuccessMessage = "practice availability successfully updated for clinician"
	CreateClinicianPracticeInformationSuccessMessage  = "practice information successfully updated for clinician"
	DeleteClinicianClinicSuccessMessage               = "clinic successfully deleted for clinician"

	// Questionnaire messages
	CreateQuestionnaireSuccessMessage = "questionnaire successfully created"
	FindQuestionnaireSuccessMessage   = "questionnaire successfully found"
	UpdateQuestionnaireSuccessMessage = "questionnaire successfully updated"
	DeleteQuestionnaireSuccessMessage = "questionnaire successfully deleted"

	// Questionnaire Response messages
	CreateQuestionnaireResponseSuccessMessage = "questionnaire response successfully created"
	FindQuestionnaireResponseSuccessMessage   = "questionnaire response successfully found"
	UpdateQuestionnaireResponseSuccessMessage = "questionnaire response successfully updated"
	DeleteQuestionnaireResponseSuccessMessage = "questionnaire response successfully deleted"

	// Patient-related messages
	CreatePatientAppointmentSuccessMessage = "appoinment successfully created for patient"

	// Auth messages
	LoginSuccessMessage          = "successfully login"
	LogoutSuccessMessage         = "successfully logout"
	ForgotPasswordSuccessMessage = "if an account with this email exists, you will receive a password reset link."
	ResetPasswordSuccessMessage  = "password already reset successfully"
)
