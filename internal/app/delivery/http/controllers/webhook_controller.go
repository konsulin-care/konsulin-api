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

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

type WebhookController struct {
	Log                           *zap.Logger
	Usecase                       webhook.Usecase
	Limiter                       *ratelimiter.HookRateLimiter
	SynchronousServiceRateLimiter *ratelimiter.ResourceLimiter
	AppConfig                     *config.InternalConfig
}

// AsyncServiceResultRequest represents the request body for async service result callback.
type AsyncServiceResultRequest struct {
	ServiceRequestID string `json:"serviceRequestId"`
	Result           string `json:"result"`
	Timestamp        string `json:"timestamp"` // RFC3339 format
}

// validate checks that all required fields are present and valid.
func (req *AsyncServiceResultRequest) validate() error {
	if strings.TrimSpace(req.ServiceRequestID) == "" {
		return exceptions.BuildNewCustomError(nil, constvars.StatusBadRequest, "serviceRequestId is required", "VALIDATION_ERROR")
	}
	if strings.TrimSpace(req.Result) == "" {
		return exceptions.BuildNewCustomError(nil, constvars.StatusBadRequest, "result is required", "VALIDATION_ERROR")
	}
	if strings.TrimSpace(req.Timestamp) == "" {
		return exceptions.BuildNewCustomError(nil, constvars.StatusBadRequest, "timestamp is required", "VALIDATION_ERROR")
	}
	// Validate timestamp format
	if _, err := time.Parse(time.RFC3339, req.Timestamp); err != nil {
		return exceptions.BuildNewCustomError(nil, constvars.StatusBadRequest, "timestamp must be in RFC3339 format", "VALIDATION_ERROR")
	}
	return nil
}

var (
	webhookControllerInstance *WebhookController
	onceWebhookController     sync.Once
)

func NewWebhookController(logger *zap.Logger, uc webhook.Usecase, limiter *ratelimiter.HookRateLimiter, syncLimiter *ratelimiter.ResourceLimiter, cfg *config.InternalConfig) *WebhookController {
	onceWebhookController.Do(func() {
		webhookControllerInstance = &WebhookController{
			Log:                           logger,
			Usecase:                       uc,
			Limiter:                       limiter,
			SynchronousServiceRateLimiter: syncLimiter,
			AppConfig:                     cfg,
		}
	})
	return webhookControllerInstance
}

// HandleSynchronousWebHook processes POST /api/v1/hook/synchronous/{service_name}
func (ctrl *WebhookController) HandleSynchronousWebHook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.BuildNewCustomError(nil, constvars.StatusMethodNotAllowed, "Only POST is allowed", "WEBHOOK_METHOD_NOT_ALLOWED"))
		return
	}

	if !strings.HasPrefix(r.Header.Get(constvars.HeaderContentType), constvars.MIMEApplicationJSON) {
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.BuildNewCustomError(nil, constvars.StatusUnsupportedMediaType, "Content-Type must be application/json", "WEBHOOK_UNSUPPORTED_MEDIA_TYPE"))
		return
	}

	serviceName := chi.URLParam(r, "service")
	if len(serviceName) == 0 || len(serviceName) > 256 {
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrClientCustomMessage(exceptions.BuildNewCustomError(nil, constvars.StatusBadRequest, "Invalid service name", "WEBHOOK_INVALID_SERVICE_NAME")))
		return
	}
	if ok, _ := regexp.MatchString(`^[A-Za-z0-9-]+$`, serviceName); !ok {
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrClientCustomMessage(exceptions.BuildNewCustomError(nil, constvars.StatusBadRequest, "Invalid service name", "WEBHOOK_INVALID_SERVICE_NAME")))
		return
	}

	raw, err := io.ReadAll(r.Body)
	if err != nil {
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrReadBody(err))
		return
	}
	defer r.Body.Close()

	var tmp map[string]interface{}
	if err := json.Unmarshal(raw, &tmp); err != nil {
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrCannotParseJSON(err))
		return
	}

	// Simple rate limiting for synchronous services
	if ctrl.SynchronousServiceRateLimiter != nil {
		limiterCfg := ctrl.AppConfig.Webhook
		eval, err := ctrl.SynchronousServiceRateLimiter.ApplyResourceLimiter(r.Context(), &ratelimiter.ApplyResourceLimiterInput{
			ResourceName:      serviceName,
			LimiterGroupName:  "HOOK_SYNCHRONOUS_SERVICE",
			WindowDurationSec: limiterCfg.SynchronousServiceWindowSeconds,
			MaxQuota:          limiterCfg.SynchronousServiceRateLimit,
		})
		if err == nil && eval != nil && !eval.Allowed {
			retryAfter := eval.RetryAfterSecs
			if retryAfter < 0 {
				retryAfter = 0
			}
			w.Header().Set(constvars.HeaderRetryAfter, fmt.Sprintf("%d", retryAfter))
			utils.BuildErrorResponse(ctrl.Log, w, exceptions.BuildNewCustomError(nil, constvars.StatusTooManyRequests, "Too many requests", "WEBHOOK_SYNC_RATE_LIMITED"))
			return
		}
	}

	ctx := context.WithValue(r.Context(), webhook.JWTForwardedFromPaymentServiceHeader, r.Header.Get(webhook.JWTForwardedFromPaymentServiceHeader))
	out, err := ctrl.Usecase.HandleSynchronousWebhookService(ctx, &webhook.HandleSynchronousWebhookServiceInput{
		ServiceName: serviceName,
		Method:      constvars.MethodPost,
		RawJSON:     raw,
	})
	if err != nil {
		utils.BuildErrorResponse(ctrl.Log, w, err)
		return
	}

	w.WriteHeader(out.StatusCode)
	_, _ = w.Write(out.Body)
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
	if ok, _ := regexp.MatchString(`^[A-Za-z0-9-]+$`, serviceName); !ok {
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
	// Derive actor ID: API key superadmin or uid or "anonymous"
	actorID := "anonymous"
	if v := r.Context().Value("api_key_auth"); v != nil {
		if b, ok := v.(bool); ok && b {
			actorID = "api-key-superadmin"
		}
	}
	if uid, ok := r.Context().Value("uid").(string); ok && uid != "" && !strings.EqualFold(uid, "anonymous") {
		actorID = uid
	}

	eval, evalErr := ctrl.Limiter.Evaluate(r.Context(), &ratelimiter.EvaluateInput{ServiceName: serviceName, NowUTC: time.Now().UTC(), ActorID: actorID})
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

	out, err := ctrl.Usecase.Enqueue(ctx, &webhook.EnqueueInput{
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

	utils.BuildSuccessResponse(w, constvars.StatusAccepted, constvars.ResponseSuccess, out)
}

func (ctrl *WebhookController) HandleAsyncServiceResultCallback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.BuildNewCustomError(nil, constvars.StatusMethodNotAllowed, "Only POST is allowed", "METHOD_NOT_ALLOWED"))
		return
	}

	// Enforce Content-Type: application/json
	if !strings.HasPrefix(r.Header.Get(constvars.HeaderContentType), constvars.MIMEApplicationJSON) {
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.BuildNewCustomError(nil, constvars.StatusUnsupportedMediaType, "Content-Type must be application/json", "UNSUPPORTED_MEDIA_TYPE"))
		return
	}

	raw, err := io.ReadAll(r.Body)
	if err != nil {
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrReadBody(err))
		return
	}
	defer r.Body.Close()

	var req AsyncServiceResultRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrCannotParseJSON(err))
		return
	}

	if err := req.validate(); err != nil {
		utils.BuildErrorResponse(ctrl.Log, w, err)
		return
	}

	timestamp, err := time.Parse(time.RFC3339, req.Timestamp)
	if err != nil {
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.BuildNewCustomError(nil, constvars.StatusBadRequest, "Invalid timestamp format", "VALIDATION_ERROR"))
		return
	}

	err = ctrl.Usecase.HandleAsyncServiceResult(r.Context(), &webhook.HandleAsyncServiceResultInput{
		ServiceRequestID: req.ServiceRequestID,
		Result:           req.Result,
		Timestamp:        timestamp,
	})
	if err != nil {
		utils.BuildErrorResponse(ctrl.Log, w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (ctrl *WebhookController) HandleGetAsyncServiceResult(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.BuildNewCustomError(nil, constvars.StatusMethodNotAllowed, "Only GET is allowed", "METHOD_NOT_ALLOWED"))
		return
	}

	id := chi.URLParam(r, "id")
	if strings.TrimSpace(id) == "" {
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.BuildNewCustomError(nil, constvars.StatusBadRequest, "id is required", "VALIDATION_ERROR"))
		return
	}

	result, err := ctrl.Usecase.GetAsyncServiceResult(r.Context(), id)
	if err != nil {
		utils.BuildErrorResponse(ctrl.Log, w, err)
		return
	}

	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.ResponseSuccess, result)
}
