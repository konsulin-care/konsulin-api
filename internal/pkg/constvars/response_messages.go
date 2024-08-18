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

	// Patient-related messages
	CreatePatientAppointmentSuccessMessage = "appoinment successfully created for patient"

	// Auth messages
	LoginSuccessMessage          = "successfully login"
	LogoutSuccessMessage         = "successfully logout"
	ForgotPasswordSuccessMessage = "reset password link already sent to your email"
	ResetPasswordSuccessMessage  = "password already reset successfully"
)
