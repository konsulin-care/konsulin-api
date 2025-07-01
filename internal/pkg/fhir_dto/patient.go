package fhir_dto

type Patient struct {
	ID           string         `json:"id,omitempty"`
	ResourceType string         `json:"resourceType,omitempty"`
	Active       bool           `json:"active,omitempty"`
	Name         []HumanName    `json:"name,omitempty"`
	Telecom      []ContactPoint `json:"telecom,omitempty"`
	Gender       string         `json:"gender,omitempty"`
	BirthDate    string         `json:"birthDate,omitempty"`
	Extension    []Extension    `json:"extension,omitempty"`
	Address      []Address      `json:"address,omitempty"`
	Identifier   []Identifier   `json:"identifier"`
}
