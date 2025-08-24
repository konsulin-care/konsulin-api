package fhir_dto

// ServiceRequest represents a minimal FHIR ServiceRequest resource needed for storage.
type ServiceRequest struct {
	ResourceType       string       `json:"resourceType"`
	ID                 string       `json:"id,omitempty"`
	Meta               Meta         `json:"meta,omitempty"`
	Status             string       `json:"status,omitempty"`
	Intent             string       `json:"intent,omitempty"`
	Requester          Reference    `json:"requester,omitempty"`
	OccurrenceDateTime string       `json:"occurrenceDateTime,omitempty"`
	Note               []Annotation `json:"note,omitempty"`
}
