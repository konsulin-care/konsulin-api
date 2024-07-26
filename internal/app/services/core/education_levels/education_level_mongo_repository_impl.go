package educationLevels

import (
	"context"
	"konsulin-service/internal/app/models"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/exceptions"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type EducationLevelMongoRepository struct {
	Collection *mongo.Collection
}

func NewEducationLevelMongoRepository(db *mongo.Database, dbName string) EducationLevelRepository {
	return &EducationLevelMongoRepository{
		Collection: db.Collection(constvars.MongoCollectionEducationLevels),
	}
}

func (repo *EducationLevelMongoRepository) FindAll(ctx context.Context) ([]models.EducationLevel, error) {
	var levels []models.EducationLevel
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

func (repo *EducationLevelMongoRepository) FindByID(ctx context.Context, educationLevelID string) (*models.EducationLevel, error) {
	var educationLevel models.EducationLevel
	objectID, err := primitive.ObjectIDFromHex(educationLevelID)
	if err != nil {
		return nil, exceptions.ErrMongoDBNotObjectID(err)
	}
	err = repo.Collection.FindOne(ctx, bson.M{"_id": objectID}).Decode(&educationLevel)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, exceptions.ErrMongoDBFindDocument(err)
	}
	return &educationLevel, nil
}

func (repo *EducationLevelMongoRepository) FindByCode(ctx context.Context, educationLevelCode string) (*models.EducationLevel, error) {
	var educationLevel models.EducationLevel
	err := repo.Collection.FindOne(ctx, bson.M{"code": educationLevelCode}).Decode(&educationLevel)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, exceptions.ErrMongoDBFindDocument(err)
	}
	return &educationLevel, nil
}
