package fhir_dto

type PractitionerRole struct {
	ResourceType  string            `json:"resourceType,omitempty"`
	ID            string            `json:"id,omitempty"`
	Practitioner  Reference         `json:"practitioner,omitempty"`
	Active        bool              `json:"active,omitempty"`
	Organization  Reference         `json:"organization,omitempty"`
	Specialty     []CodeableConcept `json:"specialty,omitempty"`
	AvailableTime []AvailableTime   `json:"availableTime,omitempty"`
	Extension     []Extension       `json:"extension,omitempty"`
	Period        Period            `json:"period,omitempty"`
}
