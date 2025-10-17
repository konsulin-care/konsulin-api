package mailer

import (
	"context"
	"encoding/json"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/exceptions"
	"sync"

	"github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"
)

var (
	mailerServiceInstance contracts.MailerService
	onceMailerService     sync.Once
	mailerServiceError    error
)

type mailerService struct {
	Channel *amqp091.Channel
	Queue   string
	Log     *zap.Logger
}

func NewMailerService(rabbitMQConnection *amqp091.Connection, logger *zap.Logger, queue string) (contracts.MailerService, error) {
	onceMailerService.Do(func() {
		channel, mailerServiceError := rabbitMQConnection.Channel()
		if mailerServiceError != nil {
			return
		}
		instance := &mailerService{
			Channel: channel,
			Queue:   queue,
			Log:     logger,
		}
		mailerServiceInstance = instance
	})
	return mailerServiceInstance, mailerServiceError
}
func (s *mailerService) SendEmail(ctx context.Context, request *requests.EmailPayload) error {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	s.Log.Info("mailerService.SendEmail called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	body, err := json.Marshal(request)
	if err != nil {
		s.Log.Error("mailerService.SendEmail error marshaling JSON",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
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

	s.Log.Info("mailerService.SendEmail publishing message",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingQueueNameKey, s.Queue),
	)
	err = s.Channel.PublishWithContext(ctx, "", s.Queue, false, false, message)
	if err != nil {
		s.Log.Error("mailerService.SendEmail error publishing message",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
			zap.String(constvars.LoggingQueueNameKey, s.Queue),
		)
		return exceptions.ErrRabbitMQPublishMessage(err, s.Queue)
	}

	s.Log.Info("mailerService.SendEmail succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingQueueNameKey, s.Queue),
	)
	return nil
}
