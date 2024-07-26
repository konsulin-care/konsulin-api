package models

import "konsulin-service/internal/pkg/dto/responses"

type EducationLevel struct {
	Code          string `json:"code" bson:"code"`
	Display       string `json:"display" bson:"display"`
	CustomDisplay string `json:"customDisplay" bson:"customDisplay"`
}

func (el EducationLevel) ConvertIntoResponse() responses.EducationLevel {
	return responses.EducationLevel{
		Name: el.CustomDisplay,
	}
}
