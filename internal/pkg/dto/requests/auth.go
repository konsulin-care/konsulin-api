package requests

type RegisterPatient struct {
	Email          string `json:"email" validate:"required,email"`
	Username       string `json:"username" validate:"required,alphanum,min=8,max=15"`
	Password       string `json:"password" validate:"password"`
	RetypePassword string `json:"retype_password"`
}

type LoginPatient struct {
	Username string `json:"username" validate:"required,min=8"`
	Password string `json:"password" validate:"required,min=8"`
}
