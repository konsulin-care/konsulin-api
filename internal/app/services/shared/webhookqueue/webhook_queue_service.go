package webhookqueue

import (
	"context"
	"encoding/json"
	"fmt"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/exceptions"
	"sync"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"
)

const (
	// Standard and DLQ names as requested.
	StandardQueueName   = "standard_webhook_service_queue"
	DeadLetterQueueName = "standard_webhook_service_dlq"
)

// WebhookQueueMessage represents the payload stored in RabbitMQ.
type WebhookQueueMessage struct {
	ID          string          `json:"id"`
	Method      string          `json:"method"`
	ServiceName string          `json:"service_name"`
	Body        json.RawMessage `json:"body"`
	FailedCount int             `json:"failed_count"`
}

// Service manages interactions with RabbitMQ queues for the webhook feature.
type Service struct {
	ch       *amqp.Channel
	log      *zap.Logger
	prefetch int
	confirms chan amqp.Confirmation
	mu       sync.Mutex
}

// NewService initializes the queue service, declares durable queues, enables confirms, and sets QoS.
func NewService(conn *amqp.Connection, log *zap.Logger, prefetch int) (*Service, error) {
	ch, err := conn.Channel()
	if err != nil {
		return nil, err
	}

	// Declare standard queue (durable)
	_, err = ch.QueueDeclare(
		StandardQueueName, // name
		true,              // durable
		false,             // autoDelete
		false,             // exclusive
		false,             // noWait
		nil,               // args
	)
	if err != nil {
		return nil, err
	}

	// Declare dead-letter queue (durable)
	_, err = ch.QueueDeclare(
		DeadLetterQueueName,
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return nil, err
	}

	// Set QoS to limit unacked deliveries in-flight
	if prefetch <= 0 {
		prefetch = 1
	}
	if err := ch.Qos(prefetch, 0, false); err != nil {
		return nil, err
	}

	// Enable publisher confirms for durability guarantees
	if err := ch.Confirm(false); err != nil {
		return nil, err
	}

	svc := &Service{
		ch:       ch,
		log:      log,
		prefetch: prefetch,
		confirms: ch.NotifyPublish(make(chan amqp.Confirmation, 1)),
	}

	return svc, nil
}

// EnqueueToWebhookServiceQueueInput defines input for enqueue operation.
type EnqueueToWebhookServiceQueueInput struct {
	Message WebhookQueueMessage
}

// EnqueueToWebhookServiceQueueOutput defines output for enqueue.
type EnqueueToWebhookServiceQueueOutput struct{}

// EnqueueToDLQInput defines input for DLQ enqueue operation.
type EnqueueToDLQInput struct {
	Message WebhookQueueMessage
}

// EnqueueToDLQOutput defines output for DLQ enqueue.
type EnqueueToDLQOutput struct{}

// ReenqueueInput defines input for reenqueueing a modified message back to the standard queue tail.
type ReenqueueInput struct {
	Message WebhookQueueMessage
}

// ReenqueueOutput defines output for reenqueue.
type ReenqueueOutput struct{}

// FetchNInput specifies the maximum number of messages to fetch.
type FetchNInput struct {
	Max int
}

// QueuedItem represents a fetched delivery and its decoded payload.
type QueuedItem struct {
	DeliveryTag uint64
	Message     WebhookQueueMessage
}

// FetchNOutput returns up to N messages.
type FetchNOutput struct {
	Items []QueuedItem
}

// AckMessageInput acknowledges a message so it is removed from the queue.
type AckMessageInput struct {
	DeliveryTag uint64
}

// AckMessageOutput is empty.
type AckMessageOutput struct{}

// Enqueue publishes a message to the standard queue with persistence and waits for confirm.
func (s *Service) Enqueue(ctx context.Context, in *EnqueueToWebhookServiceQueueInput) (*EnqueueToWebhookServiceQueueOutput, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	s.log.Info("WebhookQueue.Enqueue called", zap.String(constvars.LoggingRequestIDKey, requestID))

	body, err := json.Marshal(in.Message)
	if err != nil {
		return nil, exceptions.ErrCannotMarshalJSON(err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	msg := amqp.Publishing{
		ContentType:  constvars.MIMEApplicationJSON,
		Body:         body,
		DeliveryMode: amqp.Persistent,
	}

	if err := s.ch.PublishWithContext(ctx, "", StandardQueueName, false, false, msg); err != nil {
		return nil, exceptions.ErrRabbitMQPublishMessage(err, StandardQueueName)
	}

	select {
	case confirmed := <-s.confirms:
		if !confirmed.Ack {
			return nil, exceptions.ErrRabbitMQPublishMessage(fmt.Errorf("message not confirmed"), StandardQueueName)
		}
	case <-ctx.Done():
		return nil, exceptions.ErrRabbitMQPublishMessage(ctx.Err(), StandardQueueName)
	}
	return &EnqueueToWebhookServiceQueueOutput{}, nil
}

// Reenqueue publishes the (possibly modified) message to the tail of the standard queue and confirms.
func (s *Service) Reenqueue(ctx context.Context, in *ReenqueueInput) (*ReenqueueOutput, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	s.log.Info("WebhookQueue.Reenqueue called", zap.String(constvars.LoggingRequestIDKey, requestID))

	body, err := json.Marshal(in.Message)
	if err != nil {
		return nil, exceptions.ErrCannotMarshalJSON(err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	msg := amqp.Publishing{
		ContentType:  constvars.MIMEApplicationJSON,
		Body:         body,
		DeliveryMode: amqp.Persistent,
	}

	if err := s.ch.PublishWithContext(ctx, "", StandardQueueName, false, false, msg); err != nil {
		return nil, exceptions.ErrRabbitMQPublishMessage(err, StandardQueueName)
	}
	select {
	case confirmed := <-s.confirms:
		if !confirmed.Ack {
			return nil, exceptions.ErrRabbitMQPublishMessage(fmt.Errorf("message not confirmed"), StandardQueueName)
		}
	case <-ctx.Done():
		return nil, exceptions.ErrRabbitMQPublishMessage(ctx.Err(), StandardQueueName)
	}
	return &ReenqueueOutput{}, nil
}

// EnqueueToDeadQueue publishes the message to DLQ and confirms.
func (s *Service) EnqueueToDeadQueue(ctx context.Context, in *EnqueueToDLQInput) (*EnqueueToDLQOutput, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	s.log.Info("WebhookQueue.EnqueueToDeadQueue called", zap.String(constvars.LoggingRequestIDKey, requestID))

	body, err := json.Marshal(in.Message)
	if err != nil {
		return nil, exceptions.ErrCannotMarshalJSON(err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	msg := amqp.Publishing{
		ContentType:  constvars.MIMEApplicationJSON,
		Body:         body,
		DeliveryMode: amqp.Persistent,
	}

	if err := s.ch.PublishWithContext(ctx, "", DeadLetterQueueName, false, false, msg); err != nil {
		return nil, exceptions.ErrRabbitMQPublishMessage(err, DeadLetterQueueName)
	}
	select {
	case confirmed := <-s.confirms:
		if !confirmed.Ack {
			return nil, exceptions.ErrRabbitMQPublishMessage(fmt.Errorf("message not confirmed"), DeadLetterQueueName)
		}
	case <-ctx.Done():
		return nil, exceptions.ErrRabbitMQPublishMessage(ctx.Err(), DeadLetterQueueName)
	}
	return &EnqueueToDLQOutput{}, nil
}

// FetchN retrieves up to N messages using basic.get without auto-ack.
func (s *Service) FetchN(ctx context.Context, in *FetchNInput) (*FetchNOutput, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	s.log.Info("WebhookQueue.FetchN called", zap.String(constvars.LoggingRequestIDKey, requestID))

	n := in.Max
	if n <= 0 {
		n = 1
	}
	items := make([]QueuedItem, 0, n)

	for i := 0; i < n; i++ {
		d, ok, err := s.ch.Get(StandardQueueName, false)
		if err != nil {
			return nil, err
		}
		if !ok {
			break
		}
		var payload WebhookQueueMessage
		if err := json.Unmarshal(d.Body, &payload); err != nil {
			// If payload is invalid JSON, move to DLQ to avoid poison message loop
			_ = d.Ack(false)
			_ = s.publishRaw(ctx, DeadLetterQueueName, d.Body)
			continue
		}
		items = append(items, QueuedItem{DeliveryTag: d.DeliveryTag, Message: payload})
	}

	return &FetchNOutput{Items: items}, nil
}

// AckMessage acknowledges a message by delivery tag.
func (s *Service) AckMessage(ctx context.Context, in *AckMessageInput) (*AckMessageOutput, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	s.log.Info("WebhookQueue.AckMessage called", zap.String(constvars.LoggingRequestIDKey, requestID))
	if err := s.ch.Ack(in.DeliveryTag, false); err != nil {
		return nil, err
	}
	return &AckMessageOutput{}, nil
}

// publishRaw is a small helper to publish raw body to a queue (used for poison messages).
func (s *Service) publishRaw(ctx context.Context, queue string, body []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	msg := amqp.Publishing{ContentType: constvars.MIMEApplicationJSON, Body: body, DeliveryMode: amqp.Persistent}
	if err := s.ch.PublishWithContext(ctx, "", queue, false, false, msg); err != nil {
		return exceptions.ErrRabbitMQPublishMessage(err, queue)
	}
	select {
	case confirmed := <-s.confirms:
		if !confirmed.Ack {
			return exceptions.ErrRabbitMQPublishMessage(fmt.Errorf("message not confirmed"), queue)
		}
	case <-ctx.Done():
		return exceptions.ErrRabbitMQPublishMessage(ctx.Err(), queue)
	}
	return nil
}
