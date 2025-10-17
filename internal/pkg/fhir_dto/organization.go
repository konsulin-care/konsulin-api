package fhir_dto

type Organization struct {
	ResourceType string            `json:"resourceType,omitempty"`
	ID           string            `json:"id,omitempty"`
	Active       bool              `json:"active,omitempty"`
	Identifier   []Identifier      `json:"identifier,omitempty"`
	Type         []CodeableConcept `json:"type,omitempty"`
	Name         string            `json:"name,omitempty"`
	Alias        []string          `json:"alias,omitempty"`
	Telecom      []ContactPoint    `json:"telecom,omitempty"`
	Address      []Address         `json:"address,omitempty"`
	PartOf       Reference         `json:"partOf,omitempty"`
}
