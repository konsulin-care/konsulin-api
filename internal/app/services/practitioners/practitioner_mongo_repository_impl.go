package practitioners

import (
	"go.mongodb.org/mongo-driver/mongo"
)

type PractitionerMongoRepository struct {
	DB *mongo.Database
}

func NewPractitionerMongoRepository(db *mongo.Database, dbName string) PractitionerRepository {
	return &PractitionerMongoRepository{
		DB: db,
	}
}
