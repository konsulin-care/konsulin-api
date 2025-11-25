package fhir_dto

type Invoice struct {
	ResourceType string               `json:"resourceType"`
	ID           string               `json:"id,omitempty"`
	Meta         Meta                 `json:"meta,omitempty"`
	Identifier   []Identifier         `json:"identifier,omitempty"`
	Status       string               `json:"status,omitempty"`
	Type         *CodeableConcept     `json:"type,omitempty"`
	Subject      *Reference           `json:"subject,omitempty"`
	Participant  []InvoiceParticipant `json:"participant,omitempty"`
	Issuer       *Reference           `json:"issuer,omitempty"`
	Date         string               `json:"date,omitempty"`
	Recipient    *Reference           `json:"recipient,omitempty"`
	TotalNet     *Money               `json:"totalNet,omitempty"`
	TotalGross   *Money               `json:"totalGross,omitempty"`
	LineItem     []InvoiceLineItem    `json:"lineItem,omitempty"`
	Note         []Annotation         `json:"note,omitempty"`
}

type InvoiceParticipant struct {
	Role  *CodeableConcept `json:"role,omitempty"`
	Actor Reference        `json:"actor"`
}

type InvoiceLineItem struct {
	Sequence                  int                     `json:"sequence,omitempty"`
	ChargeItemReference       *Reference              `json:"chargeItemReference,omitempty"`
	ChargeItemCodeableConcept *CodeableConcept        `json:"chargeItemCodeableConcept,omitempty"`
	PriceComponent            []InvoicePriceComponent `json:"priceComponent,omitempty"`
}

type InvoicePriceComponent struct {
	Type   string           `json:"type,omitempty"`
	Code   *CodeableConcept `json:"code,omitempty"`
	Factor float64          `json:"factor,omitempty"`
	Amount *Money           `json:"amount,omitempty"`
}
