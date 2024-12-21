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
