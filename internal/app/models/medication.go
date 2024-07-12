package models

type Medication struct {
	ID           string                 `json:"id,omitempty" bson:"_id,omitempty"`
	Meta         Meta                   `json:"meta,omitempty" bson:"meta,omitempty"`
	Identifier   []Identifier           `json:"identifier,omitempty" bson:"identifier,omitempty"`
	Code         *CodeableConcept       `json:"code,omitempty" bson:"code,omitempty"`
	Status       string                 `json:"status,omitempty" bson:"status,omitempty"`
	Manufacturer *Reference             `json:"manufacturer,omitempty" bson:"manufacturer,omitempty"`
	Form         *CodeableConcept       `json:"form,omitempty" bson:"form,omitempty"`
	Amount       *Ratio                 `json:"amount,omitempty" bson:"amount,omitempty"`
	Ingredient   []MedicationIngredient `json:"ingredient,omitempty" bson:"ingredient,omitempty"`
	Batch        *MedicationBatch       `json:"batch,omitempty" bson:"batch,omitempty"`
}
