package database

import (
	"context"
	"fmt"
	"konsulin-service/internal/app/config"
	"log"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func NewMongoDB(driverConfig *config.DriverConfig) *mongo.Client {
	connectionString := fmt.Sprintf(
		"mongodb://%s:%s@%s:%s",
		driverConfig.MongoDB.Username,
		driverConfig.MongoDB.Password,
		driverConfig.MongoDB.Host,
		driverConfig.MongoDB.Port,
	)
	dbOptions := options.Client().ApplyURI(connectionString)
	client, err := mongo.Connect(context.TODO(), dbOptions)
	if err != nil {
		log.Fatalf("Failed to connect to mongo database: %s", err.Error())
	}
	err = client.Ping(context.TODO(), nil)
	if err != nil {
		log.Fatalf("Failed to ping or test the connection to mongo database: %s", err.Error())
	}
	log.Println("Successfully connected to mongo database")
	return client
}
