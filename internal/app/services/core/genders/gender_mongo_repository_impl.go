package genders

import (
	"context"
	"konsulin-service/internal/app/models"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/exceptions"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type GenderMongoRepository struct {
	Collection *mongo.Collection
}

func NewGenderMongoRepository(db *mongo.Client, dbName string) GenderRepository {
	return &GenderMongoRepository{
		Collection: db.Database(dbName).Collection(constvars.MongoCollectionGenders),
	}
}

func (repo *GenderMongoRepository) FindAll(ctx context.Context) ([]models.Gender, error) {
	var levels []models.Gender
	cursor, err := repo.Collection.Find(ctx, bson.M{})
	if err != nil {
		return nil, exceptions.ErrMongoDBFindDocument(err)
	}
	err = cursor.All(ctx, &levels)
	if err != nil {
		return nil, exceptions.ErrMongoDBIterateDocuments(err)
	}
	return levels, nil
}

func (repo *GenderMongoRepository) FindByID(ctx context.Context, genderID string) (*models.Gender, error) {
	var gender models.Gender
	objectID, err := primitive.ObjectIDFromHex(genderID)
	if err != nil {
		return nil, exceptions.ErrMongoDBNotObjectID(err)
	}
	err = repo.Collection.FindOne(ctx, bson.M{"_id": objectID}).Decode(&gender)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, exceptions.ErrMongoDBFindDocument(err)
	}
	return &gender, nil
}

func (repo *GenderMongoRepository) FindByCode(ctx context.Context, genderCode string) (*models.Gender, error) {
	var gender models.Gender
	err := repo.Collection.FindOne(ctx, bson.M{"code": genderCode}).Decode(&gender)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, exceptions.ErrMongoDBFindDocument(err)
	}
	return &gender, nil
}
