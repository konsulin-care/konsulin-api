package whatsapp

import (
	"context"
	"encoding/json"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/exceptions"

	"github.com/rabbitmq/amqp091-go"
)

type whatsAppService struct {
	Channel *amqp091.Channel
	Queue   string
}

func NewWhatsAppService(rabbitMQConnection *amqp091.Connection, queue string) (contracts.WhatsAppService, error) {
	channel, err := rabbitMQConnection.Channel()
	if err != nil {
		return nil, err
	}

	return &whatsAppService{
		Channel: channel,
		Queue:   queue,
	}, nil
}
func (s *whatsAppService) SendWhatsAppMessage(ctx context.Context, request *requests.WhatsAppMessage) error {
	body, err := json.Marshal(request)
	if err != nil {
		return err
	}

	headers := amqp091.Table{
		"message_type":     "JSON",
		"requeue_strategy": "DROP",
	}

	message := amqp091.Publishing{
		ContentType:  constvars.MIMEApplicationJSON,
		Body:         body,
		DeliveryMode: amqp091.Persistent,
		Priority:     0,
		Headers:      headers,
	}

	err = s.Channel.PublishWithContext(ctx, "", s.Queue, false, false, message)
	if err != nil {
		return exceptions.ErrRabbitMQPublishMessage(err, s.Queue)
	}

	return nil
}
