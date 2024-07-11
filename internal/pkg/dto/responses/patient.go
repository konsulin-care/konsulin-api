package responses

import "time"

// Patient type
type Patient struct {
	ID                   string                 `json:"id,omitempty" bson:"_id,omitempty"`
	Meta                 Meta                   `json:"meta,omitempty" bson:"meta,omitempty"`
	Identifier           []Identifier           `json:"identifier,omitempty" bson:"identifier,omitempty"`
	Active               bool                   `json:"active,omitempty" bson:"active,omitempty"`
	Name                 []HumanName            `json:"name,omitempty" bson:"name,omitempty"`
	Telecom              []ContactPoint         `json:"telecom,omitempty" bson:"telecom,omitempty"`
	Gender               string                 `json:"gender,omitempty" bson:"gender,omitempty"`
	BirthDate            string                 `json:"birthDate,omitempty" bson:"birthDate,omitempty"`
	DeceasedBoolean      *bool                  `json:"deceasedBoolean,omitempty" bson:"deceasedBoolean,omitempty"`
	DeceasedDateTime     *time.Time             `json:"deceasedDateTime,omitempty" bson:"deceasedDateTime,omitempty"`
	Address              []Address              `json:"address,omitempty" bson:"address,omitempty"`
	MaritalStatus        *CodeableConcept       `json:"maritalStatus,omitempty" bson:"maritalStatus,omitempty"`
	MultipleBirthBoolean *bool                  `json:"multipleBirthBoolean,omitempty" bson:"multipleBirthBoolean,omitempty"`
	MultipleBirthInteger *int                   `json:"multipleBirthInteger,omitempty" bson:"multipleBirthInteger,omitempty"`
	Photo                []Attachment           `json:"photo,omitempty" bson:"photo,omitempty"`
	Contact              []PatientContact       `json:"contact,omitempty" bson:"contact,omitempty"`
	Communication        []PatientCommunication `json:"communication,omitempty" bson:"communication,omitempty"`
	Extension            []Extension            `json:"extension,omitempty" bson:"extension,omitempty"`
}

type PatientContact struct {
	Relationship []CodeableConcept `json:"relationship,omitempty" bson:"relationship,omitempty"`
	Name         *HumanName        `json:"name,omitempty" bson:"name,omitempty"`
	Telecom      []ContactPoint    `json:"telecom,omitempty" bson:"telecom,omitempty"`
	Address      *Address          `json:"address,omitempty" bson:"address,omitempty"`
	Gender       *string           `json:"gender,omitempty" bson:"gender,omitempty"`
	Organization *Reference        `json:"organization,omitempty" bson:"organization,omitempty"`
	Period       *Period           `json:"period,omitempty" bson:"period,omitempty"`
}

type PatientCommunication struct {
	Language  *CodeableConcept `json:"language" bson:"language"`
	Preferred *bool            `json:"preferred,omitempty" bson:"preferred,omitempty"`
}
