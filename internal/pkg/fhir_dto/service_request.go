package fhir_dto

// CreateServiceRequestInput is the payload sent to FHIR when creating a ServiceRequest.
type CreateServiceRequestInput struct {
	ResourceType       string       `json:"resourceType"`
	Status             string       `json:"status,omitempty"`
	Intent             string       `json:"intent,omitempty"`
	Requester          Reference    `json:"requester,omitempty"`
	OccurrenceDateTime string       `json:"occurrenceDateTime,omitempty"`
	Note               []Annotation `json:"note,omitempty"`
}

// CreateServiceRequestOutput represents the ServiceRequest returned by FHIR.
type CreateServiceRequestOutput struct {
	ResourceType       string       `json:"resourceType"`
	ID                 string       `json:"id,omitempty"`
	Meta               Meta         `json:"meta,omitempty"`
	Status             string       `json:"status,omitempty"`
	Intent             string       `json:"intent,omitempty"`
	Requester          Reference    `json:"requester,omitempty"`
	OccurrenceDateTime string       `json:"occurrenceDateTime,omitempty"`
	Note               []Annotation `json:"note,omitempty"`
}

// GetServiceRequestOutput represents the response when fetching a specific version
// of a ServiceRequest resource from FHIR.
type GetServiceRequestOutput struct {
	ResourceType       string       `json:"resourceType"`
	ID                 string       `json:"id,omitempty"`
	Meta               Meta         `json:"meta,omitempty"`
	Status             string       `json:"status,omitempty"`
	Intent             string       `json:"intent,omitempty"`
	Requester          Reference    `json:"requester,omitempty"`
	OccurrenceDateTime string       `json:"occurrenceDateTime,omitempty"`
	Note               []Annotation `json:"note,omitempty"`
}
