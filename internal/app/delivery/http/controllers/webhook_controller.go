package controllers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/services/core/webhook"
	"konsulin-service/internal/app/services/shared/ratelimiter"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/utils"

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
	defer func() { _ = r.Body.Close() }()

	contentType := r.Header.Get(constvars.HeaderContentType)

	// Simple rate limiting for synchronous services
	if ctrl.SynchronousServiceRateLimiter != nil {
		limiterCfg := ctrl.AppConfig.Webhook
		eval, err := ctrl.SynchronousServiceRateLimiter.ApplyResourceLimiter(r.Context(), &ratelimiter.ApplyResourceLimiterInput{
			ResourceName:      serviceName,
			LimiterGroupName:  "HOOK_SYNCHRONOUS_SERVICE",
			WindowDurationSec: limiterCfg.SynchronousServiceWindowSeconds,
			MaxQuota:          limiterCfg.SynchronousServiceRateLimit,
		})
		if err != nil {
			utils.BuildErrorResponse(ctrl.Log, w, err)
			return
		}

		if eval != nil && rejectRateLimited(ctrl.Log, w, eval.Allowed, eval.RetryAfterSecs, "WEBHOOK_SYNC_RATE_LIMITED") {
			return
		}
	}

	ctx := context.WithValue(r.Context(), webhook.ContextKeyJWTForwardedValue, r.Header.Get(webhook.JWTForwardedFromPaymentServiceHeader))
	out, err := ctrl.Usecase.HandleSynchronousWebhookService(ctx, &webhook.HandleSynchronousWebhookServiceInput{
		ServiceName: serviceName,
		Method:      constvars.MethodPost,
		Body:        raw,
		ContentType: contentType,
	})
	if err != nil {
		utils.BuildErrorResponse(ctrl.Log, w, err)
		return
	}

	w.Header().Set(constvars.HeaderContentType, out.ContentType)
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

	serviceName, err := extractServiceName(r.URL.Path)
	if err != nil {
		utils.BuildErrorResponse(ctrl.Log, w, err)
		return
	}

	raw, readErr := io.ReadAll(r.Body)
	if readErr != nil {
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrReadBody(readErr))
		return
	}
	defer func() { _ = r.Body.Close() }()

	if err := validateJSONBody(raw); err != nil {
		utils.BuildErrorResponse(ctrl.Log, w, err)
		return
	}

	actorID := deriveActorID(r.Context())
	eval, evalErr := ctrl.Limiter.Evaluate(r.Context(), &ratelimiter.EvaluateInput{ServiceName: serviceName, NowUTC: time.Now().UTC(), ActorID: actorID})
	if evalErr != nil {
		utils.BuildErrorResponse(ctrl.Log, w, evalErr)
		return
	}
	if rejectRateLimited(ctrl.Log, w, eval.Allowed, eval.RetryAfterSecs, "WEBHOOK_RATE_LIMITED") {
		return
	}

	fwd := r.Header.Get(webhook.JWTForwardedFromPaymentServiceHeader)
	ctx := context.WithValue(r.Context(), webhook.ContextKeyJWTForwardedValue, fwd)

	out, ucErr := ctrl.Usecase.Enqueue(ctx, &webhook.EnqueueInput{
		ServiceName: serviceName,
		Method:      constvars.MethodPost,
		RawJSON:     raw,
	})
	if ucErr != nil {
		utils.BuildErrorResponse(ctrl.Log, w, ucErr)
		return
	}

	utils.BuildSuccessResponse(w, constvars.StatusAccepted, constvars.ResponseSuccess, out)
}

// extractServiceName extracts the last segment of a /{prefix}/{version}/hook/{service} path.
func extractServiceName(path string) (string, error) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	serviceName := ""
	if len(parts) >= 4 {
		serviceName = parts[len(parts)-1]
	}

	if len(serviceName) == 0 || len(serviceName) > 256 {
		return "", exceptions.ErrClientCustomMessage(exceptions.BuildNewCustomError(nil, constvars.StatusBadRequest, "Invalid service name", "WEBHOOK_INVALID_SERVICE_NAME"))
	}
	if ok, _ := regexp.MatchString(`^[A-Za-z0-9-]+$`, serviceName); !ok {
		return "", exceptions.ErrClientCustomMessage(exceptions.BuildNewCustomError(nil, constvars.StatusBadRequest, "Invalid service name", "WEBHOOK_INVALID_SERVICE_NAME"))
	}
	return serviceName, nil
}

// validateJSONBody checks that the raw body is valid JSON.
func validateJSONBody(raw []byte) error {
	var tmp map[string]interface{}
	if err := json.Unmarshal(raw, &tmp); err != nil {
		// Valid JSON that isn't an object (number, string, bool, array)
		// is accepted and forwarded as-is, consistent with parseJSONBodyFields.
		if json.Valid(raw) {
			return nil
		}
		return exceptions.ErrCannotParseJSON(err)
	}
	return nil
}

// deriveActorID extracts the actor identifier from context for rate limiting.
func deriveActorID(ctx context.Context) string {
	if v := ctx.Value(constvars.CONTEXT_API_KEY_AUTH); v != nil {
		if b, ok := v.(bool); ok && b {
			return "api-key-superadmin"
		}
	}
	if uid, ok := ctx.Value(constvars.CONTEXT_UID).(string); ok && uid != "" && !strings.EqualFold(uid, "anonymous") {
		return uid
	}
	return "anonymous"
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
	defer func() { _ = r.Body.Close() }()

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
