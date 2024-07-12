package responses

import "time"

type Observation struct {
	ID                   string            `json:"id,omitempty" bson:"_id,omitempty"`
	Meta                 Meta              `json:"meta,omitempty" bson:"meta,omitempty"`
	Identifier           []Identifier      `json:"identifier,omitempty" bson:"identifier,omitempty"`
	Status               string            `json:"status,omitempty" bson:"status,omitempty"`
	Category             []CodeableConcept `json:"category,omitempty" bson:"category,omitempty"`
	Code                 CodeableConcept   `json:"code" bson:"code"`
	Subject              Reference         `json:"subject,omitempty" bson:"subject,omitempty"`
	EffectiveDateTime    *time.Time        `json:"effectiveDateTime,omitempty" bson:"effectiveDateTime,omitempty"`
	Issued               *time.Time        `json:"issued,omitempty" bson:"issued,omitempty"`
	ValueQuantity        *Quantity         `json:"valueQuantity,omitempty" bson:"valueQuantity,omitempty"`
	ValueCodeableConcept *CodeableConcept  `json:"valueCodeableConcept,omitempty" bson:"valueCodeableConcept,omitempty"`
	Interpretation       []CodeableConcept `json:"interpretation,omitempty" bson:"interpretation,omitempty"`
	Note                 []Annotation      `json:"note,omitempty" bson:"note,omitempty"`
}
