package mailer

import (
	"context"
	"encoding/json"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/exceptions"
	"regexp"

	"github.com/rabbitmq/amqp091-go"
)

type mailerService struct {
	Channel *amqp091.Channel
	Queue   string
}

func NewMailerService(rabbitMQConnection *amqp091.Connection, queue string) (MailerService, error) {
	channel, err := rabbitMQConnection.Channel()
	if err != nil {
		return nil, err
	}

	return &mailerService{
		Channel: channel,
		Queue:   queue,
	}, nil
}
func (s *mailerService) SendEmail(ctx context.Context, request *requests.EmailPayload) error {
	body, err := json.Marshal(request)
	if err != nil {
		return exceptions.ErrCannotMarshalJSON(err)
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

func (svc *mailerService) ValidateEmail(email string) bool {
	re := regexp.MustCompile(constvars.RegexEmail)
	return re.MatchString(email)
}
