package fhir_dto

type PaymentNoticeStatus string

const (
	PaymentNoticeStatusActive         PaymentNoticeStatus = "active"
	PaymentNoticeStatusCancelled      PaymentNoticeStatus = "cancelled"
	PaymentNoticeStatusDraft          PaymentNoticeStatus = "draft"
	PaymentNoticeStatusEnteredInError PaymentNoticeStatus = "entered-in-error"
)

type PaymentNotice struct {
	ResourceType string              `json:"resourceType"`
	ID           string              `json:"id,omitempty"`
	Meta         Meta                `json:"meta,omitempty"`
	Identifier   []Identifier        `json:"identifier,omitempty"`
	Status       PaymentNoticeStatus `json:"status"`
	Request      *Reference          `json:"request,omitempty"`
	Response     *Reference          `json:"response,omitempty"`
	Created      string              `json:"created"`
	Provider     *Reference          `json:"provider,omitempty"`
	Payment      *Reference          `json:"payment,omitempty"`
	PaymentDate  string              `json:"paymentDate,omitempty"`
	Payee        *Reference          `json:"payee,omitempty"`
	Recipient    *Reference          `json:"recipient,omitempty"`
	Amount       Money               `json:"amount"`
	PaymentStatus *CodeableConcept   `json:"paymentStatus,omitempty"`
}

