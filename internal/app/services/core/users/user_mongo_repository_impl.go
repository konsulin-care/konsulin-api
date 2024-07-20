package users

import (
	"context"
	"konsulin-service/internal/app/models"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/exceptions"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type UserMongoRepository struct {
	Collection *mongo.Collection
}

func NewUserMongoRepository(db *mongo.Database, dbName string) UserRepository {
	return &UserMongoRepository{
		Collection: db.Collection(constvars.MongoCollectionUsers),
	}
}

func (repo *UserMongoRepository) CreateUser(ctx context.Context, userModel *models.User) (userID string, err error) {
	result, err := repo.Collection.InsertOne(ctx, userModel)
	if err != nil {
		return "", exceptions.ErrMongoDBInsertDocument(err)
	}
	return result.InsertedID.(primitive.ObjectID).Hex(), nil
}

func (r *UserMongoRepository) FindByEmail(ctx context.Context, email string) (*models.User, error) {
	var user models.User
	err := r.Collection.FindOne(ctx, bson.M{"email": email}).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, exceptions.ErrMongoDBFindDocument(err)
		}
		return nil, exceptions.ErrMongoDBFindDocument(err)
	}
	return &user, nil
}

func (r *UserMongoRepository) FindByUsername(ctx context.Context, username string) (*models.User, error) {
	var user models.User
	err := r.Collection.FindOne(ctx, bson.M{"username": username}).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, exceptions.ErrMongoDBFindDocument(err)
		}
		return nil, exceptions.ErrMongoDBFindDocument(err)
	}
	return &user, nil
}

func (r *UserMongoRepository) GetUserByID(ctx context.Context, userID string) (*models.User, error) {
	var user models.User
	objectID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return nil, exceptions.ErrMongoDBNotObjectID(err)
	}
	err = r.Collection.FindOne(ctx, bson.M{"_id": objectID}).Decode(&user)
	if err != nil {
		return nil, exceptions.ErrMongoDBFindDocument(err)
	}
	return &user, nil
}

func (r *UserMongoRepository) UpdateUser(ctx context.Context, user *models.User) error {
	filter := bson.M{"_id": user.ID}
	update := bson.M{"$set": user}
	_, err := r.Collection.UpdateOne(ctx, filter, update, options.Update().SetUpsert(true))
	if err != nil {
		return exceptions.ErrMongoDBUpdateDocument(err)
	}
	return nil
}

func (r *UserMongoRepository) FindByResetToken(ctx context.Context, token string) (*models.User, error) {
	var user models.User
	filter := bson.M{"resetToken": token}
	err := r.Collection.FindOne(ctx, filter).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, exceptions.ErrMongoDBFindDocument(err)
		}
		return nil, err
	}
	return &user, nil
}
