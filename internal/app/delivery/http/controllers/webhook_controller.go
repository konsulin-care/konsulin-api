package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/services/core/webhook"
	"konsulin-service/internal/app/services/shared/ratelimiter"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/utils"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

type WebhookController struct {
	Log       *zap.Logger
	Usecase   webhook.Usecase
	Limiter   *ratelimiter.HookRateLimiter
	AppConfig *config.InternalConfig
}

var (
	webhookControllerInstance *WebhookController
	onceWebhookController     sync.Once
)

func NewWebhookController(logger *zap.Logger, uc webhook.Usecase, limiter *ratelimiter.HookRateLimiter, cfg *config.InternalConfig) *WebhookController {
	onceWebhookController.Do(func() {
		webhookControllerInstance = &WebhookController{Log: logger, Usecase: uc, Limiter: limiter, AppConfig: cfg}
	})
	return webhookControllerInstance
}

// HandleEnqueueWebHook processes POST /api/v1/hook/{service_name}
func (ctrl *WebhookController) HandleEnqueueWebHook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.BuildNewCustomError(nil, constvars.StatusMethodNotAllowed, "Only POST is allowed", "WEBHOOK_METHOD_NOT_ALLOWED"))
		return
	}

	// Enforce Content-Type: application/json
	if !strings.HasPrefix(r.Header.Get(constvars.HeaderContentType), constvars.MIMEApplicationJSON) {
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.BuildNewCustomError(nil, constvars.StatusUnsupportedMediaType, "Content-Type must be application/json", "WEBHOOK_UNSUPPORTED_MEDIA_TYPE"))
		return
	}

	// Extract service_name from URL path (simple split, route attaches this under /hook/{service})
	// Expecting router to mount as /{prefix}/{version}/hook/{service}
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	serviceName := ""
	if len(parts) >= 4 {
		serviceName = parts[len(parts)-1]
	}

	// Validate serviceName: alphanumeric only; max len 256
	if len(serviceName) == 0 || len(serviceName) > 256 {
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrClientCustomMessage(exceptions.BuildNewCustomError(nil, constvars.StatusBadRequest, "Invalid service name", "WEBHOOK_INVALID_SERVICE_NAME")))
		return
	}
	if ok, _ := regexp.MatchString(`^[A-Za-z0-9]+$`, serviceName); !ok {
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrClientCustomMessage(exceptions.BuildNewCustomError(nil, constvars.StatusBadRequest, "Invalid service name", "WEBHOOK_INVALID_SERVICE_NAME")))
		return
	}

	// Read raw JSON body
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrReadBody(err))
		return
	}
	defer r.Body.Close()

	// Basic JSON check
	var tmp map[string]interface{}
	if err := json.Unmarshal(raw, &tmp); err != nil {
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrCannotParseJSON(err))
		return
	}

	// Rate limit evaluation before enqueue
	eval, evalErr := ctrl.Limiter.Evaluate(r.Context(), &ratelimiter.EvaluateInput{ServiceName: serviceName, NowUTC: time.Now().UTC()})
	if evalErr != nil {
		utils.BuildErrorResponse(ctrl.Log, w, evalErr)
		return
	}
	if !eval.Allowed {
		retryAfter := eval.RetryAfterSecs
		if retryAfter < 0 {
			retryAfter = 0
		}
		w.Header().Set(constvars.HeaderRetryAfter, fmt.Sprintf("%d", retryAfter))
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.BuildNewCustomError(nil, constvars.StatusTooManyRequests, "Too many requests", "WEBHOOK_RATE_LIMITED"))
		return
	}

	// Attach forwarded JWT header (if any) into context for usecase auth evaluation
	fwd := r.Header.Get(webhook.JWTForwardedFromPaymentServiceHeader)
	ctx := context.WithValue(r.Context(), webhook.JWTForwardedFromPaymentServiceHeader, fwd)

	_, err = ctrl.Usecase.Enqueue(ctx, &webhook.EnqueueInput{
		ServiceName: serviceName,
		Method:      constvars.MethodPost,
		RawJSON:     raw,
	})
	if err != nil {
		// If 429 expected, return with Retry-After set by caller requirement later in router or here we can set minimal
		// Prefer standard error handling
		utils.BuildErrorResponse(ctrl.Log, w, err)
		return
	}

	w.Header().Set(constvars.HeaderContentType, constvars.MIMEApplicationJSON)
	w.WriteHeader(constvars.StatusAccepted)
	w.Write([]byte(`{"success":true}`))
}
