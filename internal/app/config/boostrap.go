package config

import (
	"context"
	"database/sql"
	"log"

	"github.com/go-chi/chi/v5"
	"github.com/minio/minio-go/v7"
	"github.com/rabbitmq/amqp091-go"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

type Bootstrap struct {
	Router         *chi.Mux
	MongoDB        *mongo.Client
	PostgresDB     *sql.DB
	Redis          *redis.Client
	Logger         *zap.Logger
	RabbitMQ       *amqp091.Connection
	Minio          *minio.Client
	InternalConfig *InternalConfig
}

func (b *Bootstrap) Shutdown(ctx context.Context) error {
	err := b.MongoDB.Disconnect(ctx)
	if err != nil {
		return err
	}
	log.Println("Successfully disconnected with MongoDB")

	err = b.PostgresDB.Close()
	if err != nil {
		return err
	}
	log.Println("Successfully closing PostgresDB connection")

	err = b.Redis.Close()
	if err != nil {
		return err
	}
	log.Println("Successfully closing Redis")

	err = b.RabbitMQ.Close()
	if err != nil {
		return err
	}
	log.Println("Successfully closing RabbitMQ")

	err = b.Logger.Sync()
	if err != nil {
		return err
	}
	log.Println("Successfully closing Logger")

	return nil
}
