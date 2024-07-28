package whatsapp

import (
	"context"
	"encoding/json"
	"fmt"
	"konsulin-service/internal/pkg/dto/requests"

	"github.com/rabbitmq/amqp091-go"
)

type whatsAppService struct {
	Channel *amqp091.Channel
	Queue   string
}

func NewWhatsAppService(rabbitMQConnection *amqp091.Connection, queue string) (WhatsAppService, error) {
	channel, err := rabbitMQConnection.Channel()
	if err != nil {
		return nil, err
	}

	return &whatsAppService{
		Channel: channel,
		Queue:   queue,
	}, nil
}
func (svc *whatsAppService) SendMessage(ctx context.Context, request *requests.WhatsAppMessage) error {
	body, err := json.Marshal(request)
	if err != nil {
		return err
	}

	err = svc.Channel.PublishWithContext(ctx,
		"",
		svc.Queue,
		false,
		false,
		amqp091.Publishing{
			ContentType: "application/json",
			Body:        body,
		})
	if err != nil {
		return fmt.Errorf("failed to publish message: %w", err)
	}

	return nil
}
