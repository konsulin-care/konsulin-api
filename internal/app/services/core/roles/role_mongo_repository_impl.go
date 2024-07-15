package roles

import (
	"context"
	"konsulin-service/internal/app/models"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/exceptions"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type RoleMongoRepository struct {
	Collection *mongo.Collection
}

func NewRoleMongoRepository(db *mongo.Database, dbName string) RoleRepository {
	return &RoleMongoRepository{
		Collection: db.Collection(constvars.MongoCollectionRoles),
	}
}

func (repo *RoleMongoRepository) CreateRole(ctx context.Context, entityRole *models.Role) (roleID string, err error) {
	result, err := repo.Collection.InsertOne(ctx, entityRole)
	if err != nil {
		return "", exceptions.ErrMongoDBInsertDocument(err)
	}
	return result.InsertedID.(primitive.ObjectID).Hex(), nil
}

func (r *RoleMongoRepository) FindByEmail(ctx context.Context, email string) (*models.Role, error) {
	role := new(models.Role)
	err := r.Collection.FindOne(ctx, bson.M{"email": email}).Decode(&role)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, exceptions.ErrMongoDBFindDocument(err)
	}
	return role, nil
}

func (r *RoleMongoRepository) FindByName(ctx context.Context, roleName string) (*models.Role, error) {
	role := new(models.Role)
	err := r.Collection.FindOne(ctx, bson.M{"name": roleName}).Decode(&role)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, exceptions.ErrMongoDBFindDocument(err)
	}
	return role, nil
}

func (r *RoleMongoRepository) FindRoleByID(ctx context.Context, roleID string) (*models.Role, error) {
	role := new(models.Role)
	objectID, err := primitive.ObjectIDFromHex(roleID)
	if err != nil {
		return nil, exceptions.ErrMongoDBNotObjectID(err)
	}
	err = r.Collection.FindOne(ctx, bson.M{"_id": objectID}).Decode(&role)
	if err != nil {
		return nil, exceptions.ErrMongoDBFindDocument(err)
	}
	return role, nil
}

func (r *RoleMongoRepository) UpdateRole(ctx context.Context, roleID string, updateData map[string]interface{}) error {
	objectID, err := primitive.ObjectIDFromHex(roleID)
	if err != nil {
		return exceptions.ErrMongoDBNotObjectID(err)
	}
	_, err = r.Collection.UpdateOne(ctx, bson.M{"_id": objectID}, bson.M{"$set": updateData})
	if err != nil {
		return exceptions.ErrMongoDBUpdateDocument(err)
	}
	return err
}

func (r *RoleMongoRepository) GetAllRoles(ctx context.Context) ([]models.Role, error) {
	var roles []models.Role
	cursor, err := r.Collection.Find(ctx, bson.M{})
	if err != nil {
		return nil, exceptions.ErrMongoDBFindDocument(err)
	}

	err = cursor.All(ctx, &roles)
	if err != nil {
		return nil, exceptions.ErrMongoDBIterateDocuments(err)
	}
	return roles, nil
}
