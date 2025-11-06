package fhir_dto

type PaymentReconciliationStatus string

const (
	PaymentReconciliationStatusActive         PaymentReconciliationStatus = "active"
	PaymentReconciliationStatusCancelled      PaymentReconciliationStatus = "cancelled"
	PaymentReconciliationStatusDraft          PaymentReconciliationStatus = "draft"
	PaymentReconciliationStatusEnteredInError PaymentReconciliationStatus = "entered-in-error"
)

type PaymentReconciliationOutcome string

const (
	PaymentReconciliationOutcomeQueued   PaymentReconciliationOutcome = "queued"
	PaymentReconciliationOutcomeComplete PaymentReconciliationOutcome = "complete"
	PaymentReconciliationOutcomeError    PaymentReconciliationOutcome = "error"
	PaymentReconciliationOutcomePartial  PaymentReconciliationOutcome = "partial"
)

type PaymentReconciliation struct {
	ResourceType      string                       `json:"resourceType"`
	ID                string                       `json:"id,omitempty"`
	Meta              Meta                         `json:"meta,omitempty"`
	Identifier        []Identifier                 `json:"identifier,omitempty"`
	Status            PaymentReconciliationStatus  `json:"status"`
	Period            *Period                      `json:"period,omitempty"`
	Created           string                       `json:"created"`
	PaymentIssuer     *Reference                   `json:"paymentIssuer,omitempty"`
	Request           *Reference                   `json:"request,omitempty"`
	Requestor         *Reference                   `json:"requestor,omitempty"`
	Outcome           PaymentReconciliationOutcome `json:"outcome,omitempty"`
	Disposition       string                       `json:"disposition,omitempty"`
	PaymentDate       string                       `json:"paymentDate"`
	PaymentAmount     Money                        `json:"paymentAmount"`
	PaymentIdentifier *Identifier                  `json:"paymentIdentifier,omitempty"`
	Detail            []PaymentReconciliationDetail `json:"detail,omitempty"`
	FormCode          *CodeableConcept             `json:"formCode,omitempty"`
	ProcessNote       []PaymentReconciliationProcessNote `json:"processNote,omitempty"`
}

type PaymentReconciliationDetail struct {
	Identifier  *Identifier      `json:"identifier,omitempty"`
	Predecessor *Identifier      `json:"predecessor,omitempty"`
	Type        CodeableConcept  `json:"type"`
	Request     *Reference       `json:"request,omitempty"`
	Submitter   *Reference       `json:"submitter,omitempty"`
	Response    *Reference       `json:"response,omitempty"`
	Date        string           `json:"date,omitempty"`
	Responsible *Reference       `json:"responsible,omitempty"`
	Payee       *Reference       `json:"payee,omitempty"`
	Amount      *Money           `json:"amount,omitempty"`
}

type PaymentReconciliationProcessNote struct {
	Type string `json:"type,omitempty"`
	Text string `json:"text,omitempty"`
}

