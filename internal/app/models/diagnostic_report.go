package models

import "time"

type DiagnosticReport struct {
	ID                 string                  `json:"id,omitempty" bson:"_id,omitempty"`
	Meta               Meta                    `json:"meta,omitempty" bson:"meta,omitempty"`
	Identifier         []Identifier            `json:"identifier,omitempty" bson:"identifier,omitempty"`
	Status             string                  `json:"status" bson:"status"`
	Category           []CodeableConcept       `json:"category,omitempty" bson:"category,omitempty"`
	Code               CodeableConcept         `json:"code" bson:"code"`
	Subject            Reference               `json:"subject,omitempty" bson:"subject,omitempty"`
	Encounter          *Reference              `json:"encounter,omitempty" bson:"encounter,omitempty"`
	EffectiveDateTime  *time.Time              `json:"effectiveDateTime,omitempty" bson:"effectiveDateTime,omitempty"`
	EffectivePeriod    *Period                 `json:"effectivePeriod,omitempty" bson:"effectivePeriod,omitempty"`
	Issued             *time.Time              `json:"issued,omitempty" bson:"issued,omitempty"`
	Performer          []Reference             `json:"performer,omitempty" bson:"performer,omitempty"`
	ResultsInterpreter []Reference             `json:"resultsInterpreter,omitempty" bson:"resultsInterpreter,omitempty"`
	Specimen           []Reference             `json:"specimen,omitempty" bson:"specimen,omitempty"`
	Result             []Reference             `json:"result,omitempty" bson:"result,omitempty"`
	ImagingStudy       []Reference             `json:"imagingStudy,omitempty" bson:"imagingStudy,omitempty"`
	Media              []DiagnosticReportMedia `json:"media,omitempty" bson:"media,omitempty"`
	Conclusion         *string                 `json:"conclusion,omitempty" bson:"conclusion,omitempty"`
	ConclusionCode     []CodeableConcept       `json:"conclusionCode,omitempty" bson:"conclusionCode,omitempty"`
	PresentedForm      []Attachment            `json:"presentedForm,omitempty" bson:"presentedForm,omitempty"`
}
