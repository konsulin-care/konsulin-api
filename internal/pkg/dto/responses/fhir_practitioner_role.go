package responses

type PractitionerRole struct {
	ResourceType string    `json:"resourceType"`
	ID           string    `json:"id"`
	Practitioner Reference `json:"practitioner"`
	Organization Reference `json:"organization"`
}
