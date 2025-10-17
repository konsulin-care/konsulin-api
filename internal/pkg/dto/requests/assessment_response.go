package requests

type FindAllAssessmentResponse struct {
	PatientID    string
	AssessmentID string
	SessionData  string
}
type CreateAssesmentResponse struct {
	SessionData           string
	RespondentType        string                 `json:"respondent_type" validate:"required,oneof=guest user"`
	QuestionnaireResponse map[string]interface{} `json:"questionnaire_response"`
}
