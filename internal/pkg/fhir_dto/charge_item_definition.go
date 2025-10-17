package fhir_dto

type ChargeItemDefinition struct {
	ResourceType      string                    `json:"resourceType" validate:"required"`
	ID                string                    `json:"id,omitempty"`
	Meta              *Meta                     `json:"meta,omitempty"`
	ImplicitRules     string                    `json:"implicitRules,omitempty"`
	Language          string                    `json:"language,omitempty"`
	Text              *Narrative                `json:"text,omitempty"`
	Extension         []Extension               `json:"extension,omitempty"`
	ModifierExtension []Extension               `json:"modifierExtension,omitempty"`
	Url               string                    `json:"url,omitempty"`
	Identifier        []Identifier              `json:"identifier,omitempty"`
	Version           string                    `json:"version,omitempty"`
	Name              string                    `json:"name,omitempty"`
	Title             string                    `json:"title,omitempty"`
	DerivedFromUri    []string                  `json:"derivedFromUri,omitempty"`
	PartOf            []string                  `json:"partOf,omitempty"`
	Replaces          []string                  `json:"replaces,omitempty"`
	Status            string                    `json:"status" validate:"required"`
	Experimental      bool                      `json:"experimental,omitempty"`
	Date              string                    `json:"date,omitempty"`
	Publisher         string                    `json:"publisher,omitempty"`
	Contact           []ContactDetail           `json:"contact,omitempty"`
	Description       string                    `json:"description,omitempty"`
	UseContext        []UsageContext            `json:"useContext,omitempty"`
	Jurisdiction      []CodeableConcept         `json:"jurisdiction,omitempty"`
	ApprovalDate      string                    `json:"approvalDate,omitempty"`
	LastReviewDate    string                    `json:"lastReviewDate,omitempty"`
	EffectivePeriod   *Period                   `json:"effectivePeriod,omitempty"`
	Code              *CodeableConcept          `json:"code,omitempty"`
	Instance          []Reference               `json:"instance,omitempty"`
	Applicability     []ChargeItemApplicability `json:"applicability,omitempty"`
	PropertyGroup     []ChargeItemPropertyGroup `json:"propertyGroup,omitempty"`
}

type ChargeItemApplicability struct {
	Description string `json:"description,omitempty"`
	Language    string `json:"language,omitempty"`
	Reference   string `json:"reference,omitempty"`
	Expression  string `json:"expression,omitempty"`
}

type ChargeItemPropertyGroup struct {
	Extension         []Extension                `json:"extension,omitempty"`
	ModifierExtension []Extension                `json:"modifierExtension,omitempty"`
	PriceComponent    []ChargeItemPriceComponent `json:"priceComponent,omitempty"`
}

type ChargeItemPriceComponent struct {
	Type   string           `json:"type" validate:"required"`
	Code   *CodeableConcept `json:"code,omitempty"`
	Factor float64          `json:"factor,omitempty"`
	Amount *Money           `json:"amount,omitempty"`
}
