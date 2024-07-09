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

type HumanName struct {
	Use    string   `json:"use"`
	Family string   `json:"family"`
	Given  []string `json:"given"`
}

type ContactPoint struct {
	System string `json:"system"`
	Value  string `json:"value"`
	Use    string `json:"use"`
}

type Extension struct {
	Url         string `json:"url"`
	ValueString string `json:"valueString,omitempty"`
	ValueCode   string `json:"valueCode,omitempty"`
	ValueInt    int    `json:"valueInt,omitempty"`
}

type Address struct {
	Use        string   `json:"use"`
	Line       []string `json:"line"`
	City       string   `json:"city"`
	State      string   `json:"state"`
	PostalCode string   `json:"postalCode"`
	Country    string   `json:"country"`
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
