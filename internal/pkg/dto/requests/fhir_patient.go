package requests

type Patient struct {
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
