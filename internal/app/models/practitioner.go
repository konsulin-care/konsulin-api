package models

type Practitioner struct {
	ID            string            `json:"id,omitempty" bson:"_id,omitempty"`
	Meta          Meta              `json:"meta,omitempty" bson:"meta,omitempty"`
	Identifier    []Identifier      `json:"identifier,omitempty" bson:"identifier,omitempty"`
	Active        bool              `json:"active,omitempty" bson:"active,omitempty"`
	Name          []HumanName       `json:"name,omitempty" bson:"name,omitempty"`
	Telecom       []ContactPoint    `json:"telecom,omitempty" bson:"telecom,omitempty"`
	Address       []Address         `json:"address,omitempty" bson:"address,omitempty"`
	Gender        string            `json:"gender,omitempty" bson:"gender,omitempty"`
	BirthDate     string            `json:"birthDate,omitempty" bson:"birthDate,omitempty"`
	Qualification []Qualification   `json:"qualification,omitempty" bson:"qualification,omitempty"`
	Communication []CodeableConcept `json:"communication,omitempty" bson:"communication,omitempty"`
	Extension     []Extension       `json:"extension,omitempty" bson:"extension,omitempty"`
}
