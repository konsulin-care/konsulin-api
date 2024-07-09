package database

import (
	"context"
	"fmt"
	"konsulin-service/internal/app/config"

	"github.com/fatih/color"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func NewMongoDB(driverConfig *config.DriverConfig, log *logrus.Logger) *mongo.Database {
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
		log.Println(color.RedString("%s", "Failed to connect to database"))
		log.Fatalln(err)
	}
	err = client.Ping(context.TODO(), nil)
	if err != nil {
		log.Println(color.RedString("%s", "Failed to connect to database"))
		log.Fatalln(err)
	}
	color.Green("Successfully connected to database")
	return client.Database(driverConfig.MongoDB.DbName)
}
