package responses

import "konsulin-service/internal/pkg/fhir_dto"

type CreateAssessmentResponse struct {
	ResponseID            string                          `json:"response_id,omitempty"`
	QuestionnaireResponse *fhir_dto.QuestionnaireResponse `json:"questionnaire_response"`
}

type AssessmentResponse struct {
	ID              string           `json:"id"`
	ParticipantName string           `json:"participant_name"`
	AssessmentTitle string           `json:"assessment_title"`
	ResultBrief     string           `json:"result_brief"`
	ResultTables    []VariableResult `json:"result_tables"`
	// QRCodeURL       string           `json:"qr_code_url"`
}

type VariableResult struct {
	VariableName string  `json:"variable_name"`
	Score        float64 `json:"score"`
}
