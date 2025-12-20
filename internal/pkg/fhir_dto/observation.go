package fhir_dto

type Observation struct {
	ResourceType      string          `json:"resourceType"`
	ID                string          `json:"id,omitempty"`
	Identifier        []Identifier    `json:"identifier,omitempty"`
	Status            string          `json:"status"`
	Code              CodeableConcept `json:"code"`
	Subject           Reference       `json:"subject"`
	Performer         []Reference     `json:"performer,omitempty"`
	EffectiveDateTime string          `json:"effectiveDateTime,omitempty"`
	Issued            string          `json:"issued,omitempty"`
	Component         []Component     `json:"component,omitempty"`
}
