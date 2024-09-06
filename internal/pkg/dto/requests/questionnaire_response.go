package requests

import fhir_dto "konsulin-service/internal/pkg/dto/fhir"

type CreateQuestionnaireResponse struct {
	RespondentType        string                          `json:"respondent_type" validate:"required,oneof=guest user"`
	QuestionnaireResponse *fhir_dto.QuestionnaireResponse `json:"questionnaire_response"`
}
