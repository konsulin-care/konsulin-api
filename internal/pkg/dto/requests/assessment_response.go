package requests

import "konsulin-service/internal/pkg/fhir_dto"

type FindAllAssessmentResponse struct {
	PatientID    string
	AssessmentID string
	SessionData  string
}
type CreateAssesmentResponse struct {
	RespondentType        string                          `json:"respondent_type" validate:"required,oneof=guest user"`
	QuestionnaireResponse *fhir_dto.QuestionnaireResponse `json:"questionnaire_response"`
}
