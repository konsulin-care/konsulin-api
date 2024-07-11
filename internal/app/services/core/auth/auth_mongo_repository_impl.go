package auth

import (
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/mongo"
)

type AuthMongoRepository struct {
	DB *mongo.Client
}

func NewAuthMongoRepository(db *mongo.Client, log *logrus.Logger) *AuthMongoRepository {
	return &AuthMongoRepository{
		DB: db,
	}
}
