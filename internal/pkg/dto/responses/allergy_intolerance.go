package responses

import "time"

type AllergyIntolerance struct {
	ID                 string                       `json:"id,omitempty" bson:"_id,omitempty"`
	Meta               Meta                         `json:"meta,omitempty" bson:"meta,omitempty"`
	Identifier         []Identifier                 `json:"identifier,omitempty" bson:"identifier,omitempty"`
	ClinicalStatus     *CodeableConcept             `json:"clinicalStatus,omitempty" bson:"clinicalStatus,omitempty"`
	VerificationStatus *CodeableConcept             `json:"verificationStatus,omitempty" bson:"verificationStatus,omitempty"`
	Type               string                       `json:"type,omitempty" bson:"type,omitempty"`
	Category           []string                     `json:"category,omitempty" bson:"category,omitempty"`
	Criticality        string                       `json:"criticality,omitempty" bson:"criticality,omitempty"`
	Code               *CodeableConcept             `json:"code,omitempty" bson:"code,omitempty"`
	Patient            Reference                    `json:"patient" bson:"patient"`
	OnsetDateTime      *time.Time                   `json:"onsetDateTime,omitempty" bson:"onsetDateTime,omitempty"`
	Recorder           *Reference                   `json:"recorder,omitempty" bson:"recorder,omitempty"`
	Asserter           *Reference                   `json:"asserter,omitempty" bson:"asserter,omitempty"`
	LastOccurrence     *time.Time                   `json:"lastOccurrence,omitempty" bson:"lastOccurrence,omitempty"`
	Note               []Annotation                 `json:"note,omitempty" bson:"note,omitempty"`
	Reaction           []AllergyIntoleranceReaction `json:"reaction,omitempty" bson:"reaction,omitempty"`
}
