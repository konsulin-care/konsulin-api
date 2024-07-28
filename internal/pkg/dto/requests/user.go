package requests

type UpdateProfile struct {
	Fullname       string   `json:"fullname" validate:"required"`
	Email          string   `json:"email" validate:"required,email"`
	BirthDate      string   `json:"birth_date" validate:"required"`
	WhatsAppNumber string   `json:"whatsapp_number" validate:"required"`
	Address        string   `json:"address" validate:"required,max=200"`
	Gender         string   `json:"gender" validate:"required"`
	Educations     []string `json:"educations" validate:"required"`
}
