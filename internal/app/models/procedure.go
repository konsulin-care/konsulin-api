package models

import "time"

type Procedure struct {
	ID                string                 `json:"id,omitempty" bson:"_id,omitempty"`
	Meta              Meta                   `json:"meta,omitempty" bson:"meta,omitempty"`
	Identifier        []Identifier           `json:"identifier,omitempty" bson:"identifier,omitempty"`
	Subject           Reference              `json:"subject,omitempty" bson:"subject,omitempty"`
	Status            string                 `json:"status,omitempty" bson:"status,omitempty"`
	Category          *CodeableConcept       `json:"category,omitempty" bson:"category,omitempty"`
	Code              *CodeableConcept       `json:"code,omitempty" bson:"code,omitempty"`
	PerformedDateTime *time.Time             `json:"performedDateTime,omitempty" bson:"performedDateTime,omitempty"`
	Recorder          *Reference             `json:"recorder,omitempty" bson:"recorder,omitempty"`
	Asserter          *Reference             `json:"asserter,omitempty" bson:"asserter,omitempty"`
	Performer         []ProcedurePerformer   `json:"performer,omitempty" bson:"performer,omitempty"`
	Location          *Reference             `json:"location,omitempty" bson:"location,omitempty"`
	ReasonCode        []CodeableConcept      `json:"reasonCode,omitempty" bson:"reasonCode,omitempty"`
	ReasonReference   []Reference            `json:"reasonReference,omitempty" bson:"reasonReference,omitempty"`
	Outcome           *CodeableConcept       `json:"outcome,omitempty" bson:"outcome,omitempty"`
	Report            []Reference            `json:"report,omitempty" bson:"report,omitempty"`
	Complication      []CodeableConcept      `json:"complication,omitempty" bson:"complication,omitempty"`
	FollowUp          []CodeableConcept      `json:"followUp,omitempty" bson:"followUp,omitempty"`
	Note              []Annotation           `json:"note,omitempty" bson:"note,omitempty"`
	FocalDevice       []ProcedureFocalDevice `json:"focalDevice,omitempty" bson:"focalDevice,omitempty"`
	UsedReference     []Reference            `json:"usedReference,omitempty" bson:"usedReference,omitempty"`
	UsedCode          []CodeableConcept      `json:"usedCode,omitempty" bson:"usedCode,omitempty"`
}
