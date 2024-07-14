package requests

type RegisterUser struct {
	Email          string `json:"email" validate:"required,email"`
	Username       string `json:"username" validate:"required,alphanum,min=8,max=15"`
	Password       string `json:"password" validate:"password"`
	RetypePassword string `json:"retype_password"`
	UserType       string
}

type LoginUser struct {
	Username string `json:"username" validate:"required,alphanum,min=8"`
	Password string `json:"password" validate:"required,min=8"`
	UserType string
}

type AuthorizeUser struct {
	SessionData    string
	Resource       string
	RequiredAction string
}
