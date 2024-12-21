package fhir_dto

type ChargeItem struct {
	ResourceType           string                `json:"resourceType" validate:"required"`
	ID                     string                `json:"id,omitempty"`
	Meta                   *Meta                 `json:"meta,omitempty"`
	ImplicitRules          string                `json:"implicitRules,omitempty"`
	Language               string                `json:"language,omitempty"`
	Text                   *Narrative            `json:"text,omitempty"`
	Extension              []Extension           `json:"extension,omitempty"`
	ModifierExtension      []Extension           `json:"modifierExtension,omitempty"`
	Identifier             []Identifier          `json:"identifier,omitempty"`
	DefinitionUri          []string              `json:"definitionUri,omitempty"`
	DefinitionCanonical    []string              `json:"definitionCanonical,omitempty"`
	Status                 string                `json:"status" validate:"required"`
	PartOf                 []Reference           `json:"partOf,omitempty"`
	Code                   CodeableConcept       `json:"code" validate:"required"`
	Subject                Reference             `json:"subject" validate:"required"`
	Context                *Reference            `json:"context,omitempty"`
	OccurrenceDateTime     string                `json:"occurrenceDateTime,omitempty"`
	OccurrencePeriod       *Period               `json:"occurrencePeriod,omitempty"`
	OccurrenceTiming       *Timing               `json:"occurrenceTiming,omitempty"`
	Performer              []ChargeItemPerformer `json:"performer,omitempty"`
	PerformingOrganization *Reference            `json:"performingOrganization,omitempty"`
	RequestingOrganization *Reference            `json:"requestingOrganization,omitempty"`
	CostCenter             *Reference            `json:"costCenter,omitempty"`
	Quantity               *Quantity             `json:"quantity,omitempty"`
	Bodysite               []CodeableConcept     `json:"bodysite,omitempty"`
	FactorOverride         float64               `json:"factorOverride,omitempty"`
	PriceOverride          *Money                `json:"priceOverride,omitempty"`
	OverrideReason         string                `json:"overrideReason,omitempty"`
	Enterer                *Reference            `json:"enterer,omitempty"`
	EnteredDate            string                `json:"enteredDate,omitempty"`
	Reason                 []CodeableConcept     `json:"reason,omitempty"`
	Service                []Reference           `json:"service,omitempty"`
	ProductReference       *Reference            `json:"productReference,omitempty"`
	ProductCodeableConcept *CodeableConcept      `json:"productCodeableConcept,omitempty"`
	Account                []Reference           `json:"account,omitempty"`
	Note                   []Annotation          `json:"note,omitempty"`
	SupportingInformation  []Reference           `json:"supportingInformation,omitempty"`
}

type ChargeItemPerformer struct {
	Function CodeableConcept `json:"function,omitempty"`
	Actor    Reference       `json:"actor" validate:"required"`
}
