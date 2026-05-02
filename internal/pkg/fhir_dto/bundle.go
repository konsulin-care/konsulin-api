package fhir_dto

import "encoding/json"

// BundleLink represents a FHIR bundle link (e.g. "self", "next") for pagination.
type BundleLink struct {
	Relation string `json:"relation"`
	Url      string `json:"url"`
}

type FHIRBundle struct {
	ResourceType string       `json:"resourceType"`
	ID           string       `json:"id"`
	Type         string       `json:"type"`
	Total        int          `json:"total"`
	Link         []BundleLink `json:"link,omitempty"`
	Entry        []Entry      `json:"entry"`
}

type Entry struct {
	Resource json.RawMessage `json:"resource"`
}
