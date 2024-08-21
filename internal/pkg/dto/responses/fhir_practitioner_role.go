package responses

type PractitionerRole struct {
	ResourceType  string            `json:"resourceType"`
	ID            string            `json:"id"`
	Practitioner  Reference         `json:"practitioner"`
	Organization  Reference         `json:"organization"`
	Active        bool              `json:"active"`
	Specialty     []CodeableConcept `json:"specialty"`
	AvailableTime []AvailableTime   `json:"availableTime"`
	Extension     []Extension       `json:"extension"`
}
type PractitionerRoleResponse struct {
	ResourceType  string            `json:"resourceType"`
	ID            string            `json:"id"`
	Practitioner  Reference         `json:"practitioner"`
	Organization  Reference         `json:"organization"`
	Specialty     []CodeableConcept `json:"specialty"`
	AvailableTime []AvailableTime   `json:"availableTime"`
	Extension     []Extension       `json:"extension"`
}
