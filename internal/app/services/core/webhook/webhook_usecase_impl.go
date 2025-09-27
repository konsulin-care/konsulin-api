package webhook

import (
	"context"
	"encoding/json"
	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/services/shared/jwtmanager"
	"konsulin-service/internal/app/services/shared/webhookqueue"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/exceptions"

	"github.com/google/uuid"

	"go.uber.org/zap"
)

// Usecase exposes operations for webhook service integration.
type Usecase interface {
	// Enqueue stores the incoming request in the durable queue after validation and rate limiting.
	Enqueue(ctx context.Context, in *EnqueueInput) (*EnqueueOutput, error)
}

type usecase struct {
	log        *zap.Logger
	cfg        *config.InternalConfig
	queue      *webhookqueue.Service
	jwtManager *jwtmanager.JWTManager
}

// NewUsecase creates a new webhook usecase instance.
func NewUsecase(log *zap.Logger, cfg *config.InternalConfig, queue *webhookqueue.Service, jwtMgr *jwtmanager.JWTManager) Usecase {
	return &usecase{log: log, cfg: cfg, queue: queue, jwtManager: jwtMgr}
}

// EnqueueInput captures request details for enqueueing.
type EnqueueInput struct {
	ServiceName string
	Method      string
	RawJSON     json.RawMessage
}

// EnqueueOutput is empty for now; reserved for future metadata.
type EnqueueOutput struct{}

// JWTForwardedFromPaymentServiceHeader is a special header key that will be checked to
// ensure the request comes from trusted payment service.
const JWTForwardedFromPaymentServiceHeader = "X-Forwarded-From-Payment-Service"

// PAYMENT_SERVICE_SUB is the expected JWT subject for forwarded requests from payment service
const PAYMENT_SERVICE_SUB = "payment-service"

// Enqueue validates, rate-limits, and enqueues the message.
func (u *usecase) Enqueue(ctx context.Context, in *EnqueueInput) (*EnqueueOutput, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	u.log.Info("webhook.usecase.Enqueue called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String("service_name", in.ServiceName),
		zap.String("method", in.Method),
	)

	// Auth gate: support forwarded JWT header from payment service
	forwarded := ""
	if v := ctx.Value(JWTForwardedFromPaymentServiceHeader); v != nil {
		if s, ok := v.(string); ok {
			forwarded = s
		}
	}
	if err := u.evaluateWebhookAuth(ctx, &evaluateAuthInput{ServiceName: in.ServiceName, ForwardedJWT: forwarded}); err != nil {
		return nil, err
	}

	// Only POST allowed per requirement
	if in.Method != constvars.MethodPost {
		return nil, exceptions.ErrClientCustomMessage(exceptions.BuildNewCustomError(nil, constvars.StatusMethodNotAllowed, "Only POST is allowed", "WEBHOOK_METHOD_NOT_ALLOWED"))
	}

	// Enqueue durable message
	msg := webhookqueue.WebhookQueueMessage{
		ID:          uuid.NewString(),
		Method:      in.Method,
		ServiceName: in.ServiceName,
		Body:        in.RawJSON,
		FailedCount: 0,
	}
	_, err := u.queue.Enqueue(ctx, &webhookqueue.EnqueueToWebhookServiceQueueInput{Message: msg})
	if err != nil {
		return nil, err
	}

	return &EnqueueOutput{}, nil
}
