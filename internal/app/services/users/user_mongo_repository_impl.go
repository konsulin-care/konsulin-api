package users

import (
	"context"
	"konsulin-service/internal/app/models"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/exceptions"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type UserMongoRepository struct {
	Collection *mongo.Collection
}

func NewUserMongoRepository(db *mongo.Database, dbName string) UserRepository {
	return &UserMongoRepository{
		Collection: db.Collection(constvars.MongoCollectionUsers),
	}
}

func (repo *UserMongoRepository) CreateUser(ctx context.Context, entityUser *models.User) (userID string, err error) {
	result, err := repo.Collection.InsertOne(ctx, entityUser)
	if err != nil {
		return "", exceptions.WrapWithError(err, constvars.StatusInternalServerError, constvars.ErrClientSomethingWrongWithApplication, constvars.ErrDevDBFailedToInsertDocument)
	}
	return result.InsertedID.(primitive.ObjectID).Hex(), nil
}

func (r *UserMongoRepository) FindByEmail(ctx context.Context, email string) (*models.User, error) {
	var user models.User
	err := r.Collection.FindOne(ctx, bson.M{"email": email}).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, exceptions.WrapWithError(err, constvars.StatusInternalServerError, constvars.ErrClientSomethingWrongWithApplication, constvars.ErrDevDBFailedToFindDocument)
	}
	return &user, nil
}

func (r *UserMongoRepository) FindByUsername(ctx context.Context, username string) (*models.User, error) {
	var user models.User
	err := r.Collection.FindOne(ctx, bson.M{"username": username}).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, exceptions.WrapWithError(err, constvars.StatusInternalServerError, constvars.ErrClientSomethingWrongWithApplication, constvars.ErrDevDBFailedToFindDocument)
	}
	return &user, nil
}

func (r *UserMongoRepository) GetUserByID(ctx context.Context, userID string) (*models.User, error) {
	var user models.User
	objectID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return nil, exceptions.WrapWithError(err, constvars.StatusInternalServerError, constvars.ErrClientSomethingWrongWithApplication, constvars.ErrDevDBStringNotObjectID)
	}
	err = r.Collection.FindOne(ctx, bson.M{"_id": objectID}).Decode(&user)
	if err != nil {
		return nil, exceptions.WrapWithError(err, constvars.StatusInternalServerError, constvars.ErrClientSomethingWrongWithApplication, constvars.ErrDevDBFailedToFindDocument)
	}
	return &user, nil
}

func (r *UserMongoRepository) UpdateUser(ctx context.Context, userID string, updateData map[string]interface{}) error {
	objectID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return exceptions.WrapWithError(err, constvars.StatusInternalServerError, constvars.ErrClientSomethingWrongWithApplication, constvars.ErrDevDBStringNotObjectID)
	}
	_, err = r.Collection.UpdateOne(ctx, bson.M{"_id": objectID}, bson.M{"$set": updateData})
	if err != nil {
		return exceptions.WrapWithError(err, constvars.StatusInternalServerError, constvars.ErrClientSomethingWrongWithApplication, constvars.ErrDevDBFailedToUpdateDocument)
	}
	return err
}
