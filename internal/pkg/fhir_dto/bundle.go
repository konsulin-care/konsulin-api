package fhir_dto

import "encoding/json"

type FHIRBundle struct {
	ResourceType string  `json:"resourceType"`
	ID           string  `json:"id"`
	Type         string  `json:"type"`
	Total        int     `json:"total"`
	Entry        []Entry `json:"entry"`
}

type Entry struct {
	Resource json.RawMessage `json:"resource"`
}
