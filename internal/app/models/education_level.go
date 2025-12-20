package models

import (
	"konsulin-service/internal/pkg/dto/responses"
)

type EducationLevel struct {
	ID            string `bson:"_id"`
	Code          string `bson:"code"`
	Display       string `bson:"display"`
	CustomDisplay string `bson:"customDisplay"`
}

func (el EducationLevel) ConvertIntoResponse() responses.EducationLevel {
	return responses.EducationLevel{
		Name: el.CustomDisplay,
	}
}
