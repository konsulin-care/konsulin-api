package fhir_dto

type Practitioner struct {
	ResourceType string         `json:"resourceType"`
	ID           string         `json:"id,omitempty"`
	Active       bool           `json:"active,omitempty"`
	Name         []HumanName    `json:"name,omitempty"`
	Telecom      []ContactPoint `json:"telecom,omitempty"`
	Gender       string         `json:"gender,omitempty"`
	BirthDate    string         `json:"birthDate,omitempty"`
	Address      []Address      `json:"address,omitempty"`
	Extension    []Extension    `json:"extension,omitempty"`
	Identifier   []Identifier   `json:"identifier"`
}
