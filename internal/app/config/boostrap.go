package config

import (
	"context"
	"github.com/go-chi/chi/v5"
	"github.com/rabbitmq/amqp091-go"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"log"
)

type Bootstrap struct {
	Router         *chi.Mux
	Redis          *redis.Client
	Logger         *zap.Logger
	RabbitMQ       *amqp091.Connection
	InternalConfig *InternalConfig
	DriverConfig   *DriverConfig
	// WorkerStop if set will be called during Shutdown to gracefully stop background workers
	WorkerStop     func()
	SlotWorkerStop func()
}

func (b *Bootstrap) Shutdown(ctx context.Context) error {
	if b.WorkerStop != nil {
		b.WorkerStop()
		log.Println("Successfully stopped background workers")
	}

	if b.SlotWorkerStop != nil {
		b.SlotWorkerStop()
		log.Println("Successfully stopped slot worker")
	}

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
