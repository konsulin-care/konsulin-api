package whatsapp

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

type whatsAppService struct {
	Channel *amqp091.Channel
	Queue   string
	Log     *zap.Logger
}

var (
	whatsAppServiceInstance contracts.WhatsAppService
	onceWhatsAppService     sync.Once
	whatsAppServiceError    error
)

func NewWhatsAppService(rabbitMQConnection *amqp091.Connection, logger *zap.Logger, queue string) (contracts.WhatsAppService, error) {
	onceWhatsAppService.Do(func() {
		channel, err := rabbitMQConnection.Channel()
		if err != nil {
			whatsAppServiceError = err
			return
		}
		instance := &whatsAppService{
			Channel: channel,
			Queue:   queue,
			Log:     logger,
		}
		whatsAppServiceInstance = instance
	})
	return whatsAppServiceInstance, whatsAppServiceError
}
func (s *whatsAppService) SendWhatsAppMessage(ctx context.Context, request *requests.WhatsAppMessage) error {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)

	s.Log.Info("whatsAppService.SendWhatsAppMessage called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	body, err := json.Marshal(request)
	if err != nil {
		s.Log.Error("whatsAppService.SendWhatsAppMessage error marshaling JSON",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
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

	s.Log.Info("whatsAppService.SendWhatsAppMessage publishing message",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingQueueNameKey, s.Queue),
	)

	err = s.Channel.PublishWithContext(ctx, "", s.Queue, false, false, message)
	if err != nil {
		s.Log.Error("whatsAppService.SendWhatsAppMessage error publishing message",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingQueueNameKey, s.Queue),
			zap.Error(err),
		)
		return exceptions.ErrRabbitMQPublishMessage(err, s.Queue)
	}

	s.Log.Info("whatsAppService.SendWhatsAppMessage succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingQueueNameKey, s.Queue),
	)

	return nil
}
