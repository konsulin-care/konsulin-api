package responses

import "time"

type Condition struct {
	ID                 string            `json:"id,omitempty" bson:"_id,omitempty"`
	Meta               Meta              `json:"meta,omitempty" bson:"meta,omitempty"`
	Identifier         []Identifier      `json:"identifier,omitempty" bson:"identifier,omitempty"`
	ClinicalStatus     *CodeableConcept  `json:"clinicalStatus,omitempty" bson:"clinicalStatus,omitempty"`
	VerificationStatus *CodeableConcept  `json:"verificationStatus,omitempty" bson:"verificationStatus,omitempty"`
	Category           []CodeableConcept `json:"category,omitempty" bson:"category,omitempty"`
	Severity           *CodeableConcept  `json:"severity,omitempty" bson:"severity,omitempty"`
	Code               *CodeableConcept  `json:"code,omitempty" bson:"code,omitempty"`
	BodySite           []CodeableConcept `json:"bodySite,omitempty" bson:"bodySite,omitempty"`
	Subject            Reference         `json:"subject,omitempty" bson:"subject,omitempty"`
	OnsetDateTime      *time.Time        `json:"onsetDateTime,omitempty" bson:"onsetDateTime,omitempty"`
	AbatementDateTime  *time.Time        `json:"abatementDateTime,omitempty" bson:"abatementDateTime,omitempty"`
	RecordedDate       *time.Time        `json:"recordedDate,omitempty" bson:"recordedDate,omitempty"`
	Recorder           *Reference        `json:"recorder,omitempty" bson:"recorder,omitempty"`
	Asserter           *Reference        `json:"asserter,omitempty" bson:"asserter,omitempty"`
	Note               []Annotation      `json:"note,omitempty" bson:"note,omitempty"`
}
