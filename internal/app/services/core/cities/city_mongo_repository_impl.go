package cities

import (
	"context"
	"konsulin-service/internal/app/models"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/exceptions"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type CityMongoRepository struct {
	Collection *mongo.Collection
}

func NewCityMongoRepository(db *mongo.Client, dbName string) CityRepository {
	return &CityMongoRepository{
		Collection: db.Database(dbName).Collection(constvars.MongoCollectionCities),
	}
}

func (repo *CityMongoRepository) FindAll(ctx context.Context) ([]models.City, error) {
	var levels []models.City
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

func (repo *CityMongoRepository) FindByID(ctx context.Context, cityID string) (*models.City, error) {
	var city models.City
	objectID, err := primitive.ObjectIDFromHex(cityID)
	if err != nil {
		return nil, exceptions.ErrMongoDBNotObjectID(err)
	}
	err = repo.Collection.FindOne(ctx, bson.M{"_id": objectID}).Decode(&city)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, exceptions.ErrMongoDBFindDocument(err)
	}
	return &city, nil
}
