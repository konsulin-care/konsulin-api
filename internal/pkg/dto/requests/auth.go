package requests

type RegisterUser struct {
	ResponseID     string `json:"response_id"`
	Email          string `json:"email" validate:"required,email"`
	Username       string `json:"username" validate:"required,username,min=8,max=15"`
	Password       string `json:"password" validate:"password"`
	RetypePassword string `json:"retype_password"`
}

type LoginUser struct {
	ResponseID string `json:"response_id"`
	Username   string `json:"username" validate:"required,username,min=8"`
	Password   string `json:"password" validate:"required,min=8"`
}

type AuthorizeUser struct {
	SessionData    string
	Resource       string
	RequiredAction string
}

type ForgotPassword struct {
	Email string `json:"email" validate:"required,email"`
}

type ResetPassword struct {
	Token             string `json:"token" validate:"required"`
	NewPassword       string `json:"new_password" validate:"required,min=8"`
	RetypeNewPassword string `json:"retype_new_password" validate:"required,min=8"`
	HashedNewPassword string
}

type SupertokenPasswordlessCreateMagicLink struct {
	Email string `json:"email,omitempty" validate:"omitempty,email"`
	// Phone is an international number without '+' prefix (digits only), e.g. 628111234567.
	Phone string   `json:"phoneNumber,omitempty"`
	Roles []string `json:"roles,omitempty" validate:"omitempty,dive,oneof=Patient Practitioner 'Clinic Admin' Researcher"`
	// OrganizationID is required when Clinic Admin or Researcher is present in
	// Roles; the created PractitionerRole resources link to this Organization.
	OrganizationID string `json:"organizationId,omitempty"`
	// RedirectToPath is an optional internal path the user should be redirected to after
	// successfully consuming the magic link. Must be a relative path starting with "/" and
	// must not contain a scheme or protocol-relative prefix. Max 256 characters.
	RedirectToPath string `json:"redirectToPath,omitempty"`
}

type SupertokenPasswordlessSigninupCreateCode struct {
	Email *string `json:"email" validate:"required,email"`
}
