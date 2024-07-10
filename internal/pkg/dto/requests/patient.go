package requests

type PatientFhir struct {
	ID           string         `json:"id"`
	ResourceType string         `json:"resourceType"`
	Active       bool           `json:"active"`
	Name         []HumanName    `json:"name"`
	Telecom      []ContactPoint `json:"telecom"`
	Gender       string         `json:"gender"`
	BirthDate    string         `json:"birthDate"`
	Extension    []Extension    `json:"extension"`
	Address      []Address      `json:"address"`
}

type UpdateProfile struct {
	Fullname       string `json:"fullname" validate:"required"`
	Email          string `json:"email" validate:"required,email"`
	BirthDate      string `json:"birth_date" validate:"required"`
	WhatsAppNumber string `json:"whatsapp_number" validate:"required"`
	Address        string `json:"address" validate:"required,max=200"`
	Gender         string `json:"gender" validate:"required"`
	Education      string `json:"education" validate:"required"`
}
