package fhir_dto

type QuestionnaireResponse struct {
	ResourceType  string                      `json:"resourceType"`
	ID            string                      `json:"id,omitempty"`
	Status        string                      `json:"status"`
	Questionnaire string                      `json:"questionnaire,omitempty"`
	Subject       Reference                   `json:"subject,omitempty"`
	Authored      string                      `json:"authored,omitempty"`
	Author        Reference                   `json:"author,omitempty"`
	Item          []QuestionnaireResponseItem `json:"item,omitempty"`
}

type QuestionnaireResponseItem struct {
	LinkID string                            `json:"linkId"`
	Text   string                            `json:"text,omitempty"`
	Answer []QuestionnaireResponseItemAnswer `json:"answer,omitempty"`
	Item   []QuestionnaireResponseItem       `json:"item,omitempty"`
}

type QuestionnaireResponseItemAnswer struct {
	ValueString  *string `json:"valueString,omitempty"`
	ValueCoding  *Coding `json:"valueCoding,omitempty"`
	ValueBoolean *bool   `json:"valueBoolean,omitempty"`
}
