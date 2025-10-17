package requests

type FindAllAssessment struct {
	SubjectType    string
	AssessmentType string `validate:"omitempty,oneof=research regular popular"`
}
