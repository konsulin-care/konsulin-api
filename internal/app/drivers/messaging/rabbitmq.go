package messaging

import (
	"fmt"
	"konsulin-service/internal/app/config"
	"log"

	"github.com/rabbitmq/amqp091-go"
)

func NewRabbitMQ(driverConfig *config.DriverConfig) *amqp091.Connection {
	connectionString := fmt.Sprintf(
		"amqp://%s:%s@%s:%s/",
		driverConfig.RabbitMQ.Username,
		driverConfig.RabbitMQ.Password,
		driverConfig.RabbitMQ.Host,
		driverConfig.RabbitMQ.Port,
	)
	conn, err := amqp091.Dial(connectionString)
	if err != nil {
		log.Fatalf("Failed to connect to rabbitMQ: %s", err.Error())
	}
	log.Println("Successfully connected to rabbitMQ")
	return conn
}
