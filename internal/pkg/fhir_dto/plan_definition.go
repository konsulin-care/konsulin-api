package fhir_dto

// PlanDefinition is a minimal FHIR R4 PlanDefinition DTO. Only the fields the
// backend actually consumes are modelled: the id (for existence checks and
// referral batch validation), status and name (for logging/diagnostics).
type PlanDefinition struct {
	ResourceType string `json:"resourceType"`
	ID           string `json:"id"`
	Status       string `json:"status,omitempty"`
	Name         string `json:"name,omitempty"`
}
