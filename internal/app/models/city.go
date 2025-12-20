package models

import (
	"konsulin-service/internal/pkg/dto/responses"
)

type City struct {
	ID   string `bson:"_id"`
	Name string `bson:"name"`
}

func (c City) ConvertIntoResponse() responses.City {
	return responses.City{
		Name: c.Name,
	}
}
