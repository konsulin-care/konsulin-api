package users

import (
	"context"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/app/models"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/exceptions"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type userMongoRepository struct {
	Collection *mongo.Collection
}

func NewUserMongoRepository(db *mongo.Client, dbName string) contracts.UserRepository {
	return &userMongoRepository{
		Collection: db.Database(dbName).Collection(constvars.MongoCollectionUsers),
	}
}

func (repo *userMongoRepository) GetClient(ctx context.Context) interface{} {
	return repo.Collection.Database().Client()
}

func (repo *userMongoRepository) CreateUser(ctx context.Context, userModel *models.User) (userID string, err error) {
	result, err := repo.Collection.InsertOne(ctx, userModel)
	if err != nil {
		return "", exceptions.ErrMongoDBInsertDocument(err)
	}
	return result.InsertedID.(primitive.ObjectID).Hex(), nil
}

func (r *userMongoRepository) FindByEmail(ctx context.Context, email string) (*models.User, error) {
	var user models.User
	err := r.Collection.FindOne(ctx, bson.M{"email": email}).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, exceptions.ErrMongoDBFindDocument(err)
	}
	return &user, nil
}

func (r *userMongoRepository) FindByWhatsAppNumber(ctx context.Context, whatsAppNumber string) (*models.User, error) {
	var user models.User
	err := r.Collection.FindOne(ctx, bson.M{"whatsAppNumber": whatsAppNumber}).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, exceptions.ErrMongoDBFindDocument(err)
	}
	return &user, nil
}

func (r *userMongoRepository) FindByUsername(ctx context.Context, username string) (*models.User, error) {
	var user models.User
	err := r.Collection.FindOne(ctx, bson.M{"username": username}).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, exceptions.ErrMongoDBFindDocument(err)
	}
	return &user, nil
}

func (r *userMongoRepository) FindByEmailOrUsername(ctx context.Context, email, username string) (*models.User, error) {
	var user models.User
	filter := bson.M{
		"$or": []bson.M{
			{"email": email},
			{"username": username},
		},
	}

	err := r.Collection.FindOne(ctx, filter).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, exceptions.ErrMongoDBFindDocument(err)
	}
	return &user, nil
}

func (r *userMongoRepository) FindByID(ctx context.Context, userID string) (*models.User, error) {
	var user models.User
	objectID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return nil, exceptions.ErrMongoDBNotObjectID(err)
	}
	err = r.Collection.FindOne(ctx, bson.M{"_id": objectID}).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, exceptions.ErrMongoDBFindDocument(err)
	}
	return &user, nil
}

func (r *userMongoRepository) UpdateUser(ctx context.Context, user *models.User) error {
	objectID, err := primitive.ObjectIDFromHex(user.ID)
	if err != nil {
		return exceptions.ErrMongoDBNotObjectID(err)
	}
	filter := bson.M{"_id": objectID}
	update := bson.M{"$set": user.ConvertToBsonM()}

	_, err = r.Collection.UpdateOne(ctx, filter, update, options.Update().SetUpsert(false))
	if err != nil {
		return exceptions.ErrMongoDBUpdateDocument(err)
	}
	return nil
}

func (r *userMongoRepository) FindByResetToken(ctx context.Context, token string) (*models.User, error) {
	var user models.User
	filter := bson.M{"resetToken": token}
	err := r.Collection.FindOne(ctx, filter).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

func (r *userMongoRepository) DeleteByID(ctx context.Context, userID string) error {
	objectID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return exceptions.ErrMongoDBNotObjectID(err)
	}
	filter := bson.M{"_id": objectID}
	_, err = r.Collection.DeleteOne(ctx, filter)
	if err != nil {
		return exceptions.ErrMongoDBDeleteDocument(err)
	}
	return nil
}
