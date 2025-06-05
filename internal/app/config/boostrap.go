package config

import (
	"context"
	"log"

	"github.com/go-chi/chi/v5"
	"github.com/minio/minio-go/v7"
	"github.com/rabbitmq/amqp091-go"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type Bootstrap struct {
	Router         *chi.Mux
	Redis          *redis.Client
	Logger         *zap.Logger
	RabbitMQ       *amqp091.Connection
	Minio          *minio.Client
	InternalConfig *InternalConfig
	DriverConfig   *DriverConfig
}

func (b *Bootstrap) Shutdown(ctx context.Context) error {
	err := b.Redis.Close()
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
