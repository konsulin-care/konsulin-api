package models

import "time"

type CarePlan struct {
	ID             string             `json:"id,omitempty" bson:"_id,omitempty"`
	Meta           Meta               `json:"meta,omitempty" bson:"meta,omitempty"`
	Identifier     []Identifier       `json:"identifier,omitempty" bson:"identifier,omitempty"`
	Status         string             `json:"status" bson:"status"`
	Intent         string             `json:"intent" bson:"intent"`
	Category       []CodeableConcept  `json:"category,omitempty" bson:"category,omitempty"`
	Title          string             `json:"title,omitempty" bson:"title,omitempty"`
	Description    string             `json:"description,omitempty" bson:"description,omitempty"`
	Subject        Reference          `json:"subject" bson:"subject"`
	Encounter      *Reference         `json:"encounter,omitempty" bson:"encounter,omitempty"`
	Period         *Period            `json:"period,omitempty" bson:"period,omitempty"`
	Created        *time.Time         `json:"created,omitempty" bson:"created,omitempty"`
	Author         *Reference         `json:"author,omitempty" bson:"author,omitempty"`
	Contributor    []Reference        `json:"contributor,omitempty" bson:"contributor,omitempty"`
	CareTeam       []Reference        `json:"careTeam,omitempty" bson:"careTeam,omitempty"`
	Addresses      []Reference        `json:"addresses,omitempty" bson:"addresses,omitempty"`
	SupportingInfo []Reference        `json:"supportingInfo,omitempty" bson:"supportingInfo,omitempty"`
	Goal           []Reference        `json:"goal,omitempty" bson:"goal,omitempty"`
	Activity       []CarePlanActivity `json:"activity,omitempty" bson:"activity,omitempty"`
	Note           []Annotation       `json:"note,omitempty" bson:"note,omitempty"`
}
