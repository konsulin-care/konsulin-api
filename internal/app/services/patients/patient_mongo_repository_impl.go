package patients

import (
	"go.mongodb.org/mongo-driver/mongo"
)

type PatientMongoRepository struct {
	DB *mongo.Database
}

func NewPatientMongoRepository(db *mongo.Database, dbName string) PatientRepository {
	return &PatientMongoRepository{
		DB: db,
	}
}
