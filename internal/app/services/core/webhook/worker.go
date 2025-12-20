package webhook

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/app/services/shared/jwtmanager"
	"konsulin-service/internal/app/services/shared/webhookqueue"
	"konsulin-service/internal/pkg/constvars"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"
)

// Worker periodically forwards queued webhook messages with at-least-once semantics.
type Worker struct {
	log        *zap.Logger
	cfg        *config.InternalConfig
	locker     contracts.LockerService
	queue      *webhookqueue.Service
	jwtManager *jwtmanager.JWTManager
	client     *http.Client
	stop       chan struct{}
}

// NewWorker creates a new worker instance.
func NewWorker(log *zap.Logger, cfg *config.InternalConfig, lockerSvc contracts.LockerService, queue *webhookqueue.Service, jwtMgr *jwtmanager.JWTManager) *Worker {
	timeout := time.Duration(cfg.Webhook.HTTPTimeoutInSeconds) * time.Second
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &Worker{
		log:        log,
		cfg:        cfg,
		locker:     lockerSvc,
		queue:      queue,
		jwtManager: jwtMgr,
		client:     &http.Client{Timeout: timeout},
		stop:       make(chan struct{}),
	}
}

// Start begins the ticker loop. It returns a stop function to halt execution.
func (w *Worker) Start(ctx context.Context) (stop func()) {
	ticker := time.NewTicker(time.Minute)
	stopped := make(chan struct{})

	fmt.Println("Webhook worker started")

	go func() {
		defer close(stopped)
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				return
			case <-w.stop:
				ticker.Stop()
				return
			case now := <-ticker.C:
				w.runOnce(ctx, now)
			}
		}
	}()

	return func() {
		close(w.stop)
	}
}

func (w *Worker) runOnce(ctx context.Context, now time.Time) {
	w.log.Info("webhook.worker.runOnce tick",
		zap.Time("now", now))

	// Acquire best-effort distributed lock
	nextMinute := now.Truncate(time.Minute).Add(time.Minute)
	ttl := time.Until(nextMinute) - 1*time.Second
	if ttl < 1*time.Second {
		ttl = 1 * time.Second
	}
	acquired, lockVal, err := w.locker.TryLock(ctx, "webhook:worker:lock", ttl)
	if err != nil {
		w.log.Info("worker lock attempt failed",
			zap.Error(err))
		return
	}
	if !acquired {
		w.log.Warn("worker lock not acquired; another instance is running")
		return
	}

	w.log.Info("worker lock acquired")
	defer func() {
		if err := w.locker.Unlock(ctx, "webhook:worker:lock", lockVal); err != nil {
			w.log.Error("worker unlock failed", zap.Error(err))
		}
	}()

	max := w.cfg.Webhook.MaxQueue
	if max <= 0 {
		max = 1
	}
	out, err := w.queue.FetchN(ctx, &webhookqueue.FetchNInput{Max: max})
	if err != nil {
		w.log.Info("queue.FetchN error", zap.Error(err))
		return
	}

	w.log.Info("queue.FetchN success", zap.Int("fetched_count", len(out.Items)))

	for _, item := range out.Items {
		w.processItem(ctx, item)
	}
}

func (w *Worker) processItem(ctx context.Context, item webhookqueue.QueuedItem) {
	msg := item.Message
	// Build request
	url := fmt.Sprintf("%s/%s", strings.TrimRight(w.cfg.Webhook.URL, "/"), msg.ServiceName)
	req, err := http.NewRequestWithContext(ctx, msg.Method, url, bytes.NewReader(msg.Body))
	if err != nil {
		w.log.Info("build http request failed",
			zap.String("service_name", msg.ServiceName),
			zap.String("method", msg.Method),
			zap.Error(err))
		w.requeueOnError(ctx, item, msg, err, true)
		return
	}
	req.Header.Set(constvars.HeaderContentType, constvars.MIMEApplicationJSON)

	tokenOut, err := w.jwtManager.CreateToken(ctx, &jwtmanager.CreateTokenInput{Subject: msg.ServiceName})
	if err != nil {
		w.log.Info("jwt create token failed",
			zap.String("service_name", msg.ServiceName),
			zap.Error(err))
		w.requeueOnError(ctx, item, msg, err, true)
		return
	}
	req.Header.Set(constvars.HeaderAuthorization, "Bearer "+tokenOut.Token)

	w.log.Info("forwarding webhook request",
		zap.String("service_name", msg.ServiceName),
		zap.String("message_id", msg.ID),
		zap.String("method", msg.Method),
		zap.String("url", url),
		zap.Int("failed_count", msg.FailedCount))

	resp, err := w.client.Do(req)
	if err != nil {
		w.log.Info("http request failed",
			zap.String("service_name", msg.ServiceName),
			zap.Error(err))
		w.requeueOnError(ctx, item, msg, err, true)
		return
	}
	defer resp.Body.Close()
	_, _ = io.ReadAll(resp.Body) // drain for connection reuse

	w.log.Info("webhook response received",
		zap.String("service_name", msg.ServiceName),
		zap.String("message_id", msg.ID),
		zap.Int("status_code", resp.StatusCode))
	switch resp.StatusCode {
	case http.StatusOK, http.StatusCreated:
		// success: ack and drop
		_, ackErr := w.queue.AckMessage(ctx, &webhookqueue.AckMessageInput{DeliveryTag: item.DeliveryTag})
		if ackErr != nil {
			w.log.Info("ack failed after success",
				zap.String("service_name", msg.ServiceName),
				zap.String("message_id", msg.ID),
				zap.Error(ackErr))
		}
		w.log.Info("message processed successfully; removed from queue",
			zap.String("service_name", msg.ServiceName),
			zap.String("message_id", msg.ID))
	case http.StatusUnauthorized, http.StatusForbidden:
		// requeue without increment
		if _, err := w.queue.Reenqueue(ctx, &webhookqueue.ReenqueueInput{Message: msg}); err != nil {
			w.log.Info("reenqueue failed (401/403)",
				zap.String("service_name", msg.ServiceName),
				zap.String("message_id", msg.ID),
				zap.Int("http_status", resp.StatusCode),
				zap.Error(err))
			return
		}
		_, _ = w.queue.AckMessage(ctx, &webhookqueue.AckMessageInput{DeliveryTag: item.DeliveryTag})
		w.log.Warn("received 401/403; message returned to tail without incrementing failed count",
			zap.String("service_name", msg.ServiceName),
			zap.String("message_id", msg.ID),
			zap.Int("failed_count", msg.FailedCount))
	default:
		// increment and requeue or DLQ
		msg.FailedCount++
		if msg.FailedCount >= w.cfg.Webhook.ThrottleRetry {
			if _, err := w.queue.EnqueueToDeadQueue(ctx, &webhookqueue.EnqueueToDLQInput{Message: msg}); err != nil {
				w.log.Info("enqueue to DLQ failed",
					zap.String("service_name", msg.ServiceName),
					zap.String("message_id", msg.ID),
					zap.Error(err))
				return
			}
			_, _ = w.queue.AckMessage(ctx, &webhookqueue.AckMessageInput{DeliveryTag: item.DeliveryTag})
			w.log.Info("moved message to DLQ",
				zap.String("service_name", msg.ServiceName),
				zap.String("message_id", msg.ID),
				zap.Int("failed_count", msg.FailedCount))
			return
		}
		if _, err := w.queue.Reenqueue(ctx, &webhookqueue.ReenqueueInput{Message: msg}); err != nil {
			w.log.Info("reenqueue failed (error status)",
				zap.String("service_name", msg.ServiceName),
				zap.String("message_id", msg.ID),
				zap.Error(err))
			return
		}
		_, _ = w.queue.AckMessage(ctx, &webhookqueue.AckMessageInput{DeliveryTag: item.DeliveryTag})
		w.log.Info("retryable failure; incremented failedCount and requeued",
			zap.String("service_name", msg.ServiceName),
			zap.String("message_id", msg.ID),
			zap.Int("failed_count", msg.FailedCount))
	}
}

func (w *Worker) requeueOnError(ctx context.Context, item webhookqueue.QueuedItem, msg webhookqueue.WebhookQueueMessage, err error, increment bool) {
	if increment {
		msg.FailedCount++
	}
	if msg.FailedCount >= w.cfg.Webhook.ThrottleRetry {
		if _, e := w.queue.EnqueueToDeadQueue(ctx, &webhookqueue.EnqueueToDLQInput{Message: msg}); e != nil {
			w.log.Info("enqueue to DLQ failed (network)",
				zap.String("service_name", msg.ServiceName),
				zap.String("message_id", msg.ID),
				zap.Error(e))
			return
		}
		_, _ = w.queue.AckMessage(ctx, &webhookqueue.AckMessageInput{DeliveryTag: item.DeliveryTag})
		w.log.Info("network/error; moved message to DLQ",
			zap.String("service_name", msg.ServiceName),
			zap.String("message_id", msg.ID),
			zap.Int("failed_count", msg.FailedCount))
		return
	}
	if _, e := w.queue.Reenqueue(ctx, &webhookqueue.ReenqueueInput{Message: msg}); e != nil {
		w.log.Info("reenqueue failed (network)",
			zap.String("service_name", msg.ServiceName),
			zap.String("message_id", msg.ID),
			zap.Error(e))
		return
	}
	_, _ = w.queue.AckMessage(ctx, &webhookqueue.AckMessageInput{DeliveryTag: item.DeliveryTag})
	w.log.Info("network/error; requeued message to tail",
		zap.String("service_name", msg.ServiceName),
		zap.String("message_id", msg.ID),
		zap.Int("failed_count", msg.FailedCount))
}
