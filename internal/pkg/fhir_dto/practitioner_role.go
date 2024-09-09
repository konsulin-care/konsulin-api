package fhir_dto

type PractitionerRole struct {
	ResourceType  string            `json:"resourceType"`
	ID            string            `json:"id"`
	Practitioner  Reference         `json:"practitioner"`
	Active        bool              `json:"active"`
	Organization  Reference         `json:"organization"`
	Specialty     []CodeableConcept `json:"specialty"`
	AvailableTime []AvailableTime   `json:"availableTime"`
	Extension     []Extension       `json:"extension"`
}
