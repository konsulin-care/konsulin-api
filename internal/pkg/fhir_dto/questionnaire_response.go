package fhir_dto

type QuestionnaireResponse struct {
	ResourceType  string                      `json:"resourceType"`
	ID            string                      `json:"id,omitempty"`
	Status        string                      `json:"status,omitempty"`
	Questionnaire string                      `json:"questionnaire,omitempty"`
	Subject       Reference                   `json:"subject,omitempty"`
	Authored      string                      `json:"authored,omitempty"`
	Author        Reference                   `json:"author,omitempty"`
	Identifier    *Identifier                 `json:"identifier,omitempty"`
	Item          []QuestionnaireResponseItem `json:"item,omitempty"`
}

type QuestionnaireResponseItem struct {
	LinkID string                            `json:"linkId"`
	Text   string                            `json:"text,omitempty"`
	Answer []QuestionnaireResponseItemAnswer `json:"answer,omitempty"`
	Item   []QuestionnaireResponseItem       `json:"item,omitempty"`
}
type QuestionnaireResponseItemAnswer struct {
	ValueBoolean    *bool       `json:"valueBoolean,omitempty"`
	ValueDecimal    *float64    `json:"valueDecimal,omitempty"`
	ValueInteger    *int        `json:"valueInteger,omitempty"`
	ValueDate       *string     `json:"valueDate,omitempty"`
	ValueDateTime   *string     `json:"valueDateTime,omitempty"`
	ValueTime       *string     `json:"valueTime,omitempty"`
	ValueString     *string     `json:"valueString,omitempty"`
	ValueUri        *string     `json:"valueUri,omitempty"`
	ValueAttachment *Attachment `json:"valueAttachment,omitempty"`
	ValueCoding     *Coding     `json:"valueCoding,omitempty"`
	ValueQuantity   *Quantity   `json:"valueQuantity,omitempty"`
	ValueReference  *Reference  `json:"valueReference,omitempty"`
}
