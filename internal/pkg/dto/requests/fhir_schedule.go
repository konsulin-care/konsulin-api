package requests

type Schedule struct {
	ResourceType    string            `json:"resourceType"`
	ID              string            `json:"id,omitempty"`
	Meta            Meta              `json:"meta,omitempty"`
	Identifier      []Identifier      `json:"identifier,omitempty"`
	Active          bool              `json:"active,omitempty"`
	ServiceCategory []CodeableConcept `json:"serviceCategory,omitempty"`
	ServiceType     []CodeableConcept `json:"serviceType,omitempty"`
	Specialty       []CodeableConcept `json:"specialty,omitempty"`
	Actor           []Reference       `json:"actor"`
	PlanningHorizon Period            `json:"planningHorizon,omitempty"`
	Comment         string            `json:"comment,omitempty"`
}
