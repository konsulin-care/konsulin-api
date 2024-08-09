package models

import "konsulin-service/internal/pkg/dto/responses"

type Gender struct {
	ID            string `bson:"_id"`
	Code          string `json:"code" bson:"code"`
	Display       string `json:"display" bson:"display"`
	CustomDisplay string `json:"customDisplay" bson:"customDisplay"`
}

func (g Gender) ConvertIntoResponse() responses.Gender {
	return responses.Gender{
		Name: g.CustomDisplay,
	}
}
