package responses

import "konsulin-service/internal/pkg/fhir_dto"

type CreateAssessmentResponse struct {
	ResponseID            string                          `json:"response_id,omitempty"`
	QuestionnaireResponse *fhir_dto.QuestionnaireResponse `json:"questionnaire_response"`
}
