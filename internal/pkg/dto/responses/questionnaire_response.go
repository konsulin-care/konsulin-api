package responses

import fhir_dto "konsulin-service/internal/pkg/dto/fhir"

type CreateQuestionnaireResponse struct {
	ResponseID            string                          `json:"response_id,omitempty"`
	QuestionnaireResponse *fhir_dto.QuestionnaireResponse `json:"questionnaire_response"`
}
