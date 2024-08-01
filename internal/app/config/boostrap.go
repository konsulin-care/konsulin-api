package config

import (
	"context"
	"log"

	"github.com/go-chi/chi/v5"
	"github.com/rabbitmq/amqp091-go"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

type Bootstrap struct {
	Router         *chi.Mux
	MongoDB        *mongo.Client
	Redis          *redis.Client
	Logger         *zap.Logger
	RabbitMQ       *amqp091.Connection
	InternalConfig *InternalConfig
}

func (b *Bootstrap) Shutdown(ctx context.Context) error {
	// Shutdown MongoDB
	err := b.MongoDB.Disconnect(ctx)
	if err != nil {
		return err
	}
	log.Println("Successfully disconnected with MongoDB")

	// Shutdown Redis
	err = b.Redis.Close()
	if err != nil {
		return err
	}
	log.Println("Successfully closing Redis")

	// Close RabbitMQ
	err = b.RabbitMQ.Close()
	if err != nil {
		return err
	}
	log.Println("Successfully closing RabbitMQ")

	// Sync the logger
	err = b.Logger.Sync()
	if err != nil {
		return err
	}
	log.Println("Successfully closing Logger")

	return nil
}
