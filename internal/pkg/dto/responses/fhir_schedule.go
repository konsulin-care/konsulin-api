package responses

type Schedule struct {
	ResourceType    string            `json:"resourceType" bson:"resourceType"`
	ID              string            `json:"id,omitempty" bson:"id,omitempty"`
	Meta            Meta              `json:"meta,omitempty" bson:"meta,omitempty"`
	Identifier      []Identifier      `json:"identifier,omitempty" bson:"identifier,omitempty"`
	Active          bool              `json:"active,omitempty" bson:"active,omitempty"`
	ServiceCategory []CodeableConcept `json:"serviceCategory,omitempty" bson:"serviceCategory,omitempty"`
	ServiceType     []CodeableConcept `json:"serviceType,omitempty" bson:"serviceType,omitempty"`
	Specialty       []CodeableConcept `json:"specialty,omitempty" bson:"specialty,omitempty"`
	Actor           []Reference       `json:"actor" bson:"actor"`
	PlanningHorizon Period            `json:"planningHorizon,omitempty" bson:"planningHorizon,omitempty"`
	Comment         string            `json:"comment,omitempty" bson:"comment,omitempty"`
}
