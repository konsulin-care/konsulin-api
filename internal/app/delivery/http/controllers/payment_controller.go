package controllers

import (
	"context"
	"encoding/json"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/utils"
	"net/http"
	"sync"
	"time"

	"go.uber.org/zap"
)

type PaymentController struct {
	Log            *zap.Logger
	PaymentUsecase contracts.PaymentUsecase
}

var (
	paymentControllerInstance *PaymentController
	oncePaymentController     sync.Once
)

func NewPaymentController(logger *zap.Logger, paymentUsecase contracts.PaymentUsecase) *PaymentController {
	oncePaymentController.Do(func() {
		instance := &PaymentController{
			Log:            logger,
			PaymentUsecase: paymentUsecase,
		}
		paymentControllerInstance = instance
	})
	return paymentControllerInstance
}
func (ctrl *PaymentController) PaymentRoutingCallback(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	requestID, ok := r.Context().Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	if !ok || requestID == "" {
		ctrl.Log.Error("Request ID missing from context",
			zap.String(constvars.LoggingEndpointKey, r.URL.Path),
			zap.String(constvars.LoggingMethodKey, r.Method),
			zap.String(constvars.LoggingRemoteAddrKey, r.RemoteAddr),
		)
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrMissingRequestID(nil))
		return
	}

	utils.LogSecurityEvent(ctrl.Log, "payment_callback_received", requestID, "info",
		zap.String(constvars.LoggingRemoteAddrKey, r.RemoteAddr),
		zap.String(constvars.LoggingUserAgentKey, r.UserAgent()),
	)

	ctrl.Log.Debug("Payment callback processing started",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingEndpointKey, r.URL.Path),
		zap.String(constvars.LoggingMethodKey, r.Method),
	)

	request := new(requests.PaymentRoutingCallback)
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		ctrl.Log.Error("Failed to parse payment callback request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingErrorTypeKey, "JSON parsing"),
			zap.Error(err),
		)
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrCannotParseJSON(err))
		return
	}
	ctrl.Log.Debug("Payment callback request parsed successfully",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String("payment_status", request.PaymentStatus),
	)

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	err := ctrl.PaymentUsecase.PaymentRoutingCallback(ctx, request)
	if err != nil {
		ctrl.Log.Error("Failed to process payment callback",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String("payment_status", request.PaymentStatus),
			zap.String(constvars.LoggingErrorTypeKey, "usecase error"),
			zap.Duration(constvars.LoggingDurationKey, time.Since(start)),
			zap.Error(err),
		)
		if err == context.DeadlineExceeded {
			utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrServerDeadlineExceeded(err))
			return
		}
		utils.BuildErrorResponse(ctrl.Log, w, err)
		return
	}

	utils.LogBusinessEvent(ctrl.Log, "payment_callback_processed", requestID,
		zap.String("payment_status", request.PaymentStatus),
		zap.Duration(constvars.LoggingDurationKey, time.Since(start)),
	)
	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.PaymentRoutingCallbackSuccessfullyCalled, request.PaymentStatus)
}

func (ctrl *PaymentController) XenditInvoiceCallback(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	requestID, ok := r.Context().Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	if !ok || requestID == "" {
		ctrl.Log.Error("Request ID missing from context",
			zap.String(constvars.LoggingEndpointKey, r.URL.Path),
			zap.String(constvars.LoggingMethodKey, r.Method),
			zap.String(constvars.LoggingRemoteAddrKey, r.RemoteAddr),
		)
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrMissingRequestID(nil))
		return
	}

	utils.LogSecurityEvent(ctrl.Log, "xendit_callback_received", requestID, "info",
		zap.String(constvars.LoggingRemoteAddrKey, r.RemoteAddr),
		zap.String(constvars.LoggingUserAgentKey, r.UserAgent()),
	)

	ctrl.Log.Debug("Xendit invoice callback processing started",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingEndpointKey, r.URL.Path),
		zap.String(constvars.LoggingMethodKey, r.Method),
	)

	// Parse headers
	callbackToken := r.Header.Get(string(requests.HeaderKeyCallbackToken))
	webhookID := r.Header.Get(string(requests.HeaderKeyWebhookID))

	if callbackToken == "" {
		ctrl.Log.Error("Missing x-callback-token header",
			zap.String(constvars.LoggingRequestIDKey, requestID),
		)
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.BuildNewCustomError(
			nil,
			constvars.StatusBadRequest,
			"Missing x-callback-token header",
			"x-callback-token header is required",
		))
		return
	}

	if webhookID == "" {
		ctrl.Log.Error("Missing x-webhook-id header",
			zap.String(constvars.LoggingRequestIDKey, requestID),
		)
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.BuildNewCustomError(
			nil,
			constvars.StatusBadRequest,
			"Missing x-webhook-id header",
			"x-webhook-id header is required",
		))
		return
	}

	header := &requests.XenditInvoiceCallbackHeader{
		CallbackToken: callbackToken,
		WebhookID:     webhookID,
	}

	// Parse body
	body := new(requests.XenditInvoiceCallbackBody)
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		ctrl.Log.Error("Failed to parse Xendit invoice callback request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingErrorTypeKey, "JSON parsing"),
			zap.Error(err),
		)
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrCannotParseJSON(err))
		return
	}

	ctrl.Log.Debug("Xendit invoice callback request parsed successfully",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String("invoice_id", body.ID),
		zap.String("external_id", body.ExternalID),
		zap.String("status", string(body.Status)),
	)

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	err := ctrl.PaymentUsecase.XenditInvoiceCallback(ctx, header, body)
	if err != nil {
		ctrl.Log.Error("Failed to process Xendit invoice callback",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String("invoice_id", body.ID),
			zap.String("status", string(body.Status)),
			zap.String(constvars.LoggingErrorTypeKey, "usecase error"),
			zap.Duration(constvars.LoggingDurationKey, time.Since(start)),
			zap.Error(err),
		)
		if err == context.DeadlineExceeded {
			utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrServerDeadlineExceeded(err))
			return
		}
		utils.BuildErrorResponse(ctrl.Log, w, err)
		return
	}

	utils.LogBusinessEvent(ctrl.Log, "xendit_callback_processed", requestID,
		zap.String("invoice_id", body.ID),
		zap.String("status", string(body.Status)),
		zap.Duration(constvars.LoggingDurationKey, time.Since(start)),
	)
	utils.BuildSuccessResponse(w, constvars.StatusOK, "Xendit invoice callback processed successfully", body.Status)
}

func (ctrl *PaymentController) CreatePay(w http.ResponseWriter, r *http.Request) {
	requestID, ok := r.Context().Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	if !ok || requestID == "" {
		ctrl.Log.Error("PaymentController.CreatePay requestID not found in context")
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrMissingRequestID(nil))
		return
	}
	ctrl.Log.Info("PaymentController.CreatePay called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	req := new(requests.CreatePayRequest)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ctrl.Log.Error("PaymentController.CreatePay error decoding JSON",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrCannotParseJSON(err))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	resp, err := ctrl.PaymentUsecase.CreatePay(ctx, req)
	if err != nil {
		ctrl.Log.Error("PaymentController.CreatePay error from usecase",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		if err == context.DeadlineExceeded {
			utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrServerDeadlineExceeded(err))
			return
		}
		utils.BuildErrorResponse(ctrl.Log, w, err)
		return
	}

	ctrl.Log.Info("PaymentController.CreatePay succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String("partner_trx_id", resp.PartnerTrxID),
	)
	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.ResponseSuccess, resp)
}

func (ctrl *PaymentController) HandleAppointmentPayment(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	requestID, ok := r.Context().Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	if !ok || requestID == "" {
		ctrl.Log.Error("PaymentController.HandleAppointmentPayment requestID not found in context")
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrMissingRequestID(nil))
		return
	}

	ctrl.Log.Info("PaymentController.HandleAppointmentPayment called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	req := new(requests.AppointmentPaymentRequest)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ctrl.Log.Error("PaymentController.HandleAppointmentPayment error decoding JSON",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrCannotParseJSON(err))
		return
	}

	if err := req.Validate(); err != nil {
		ctrl.Log.Error("PaymentController.HandleAppointmentPayment validation failed",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.BuildNewCustomError(
			err,
			constvars.StatusBadRequest,
			err.Error(),
			"validation error",
		))
		return
	}

	resp, err := ctrl.PaymentUsecase.HandleAppointmentPayment(r.Context(), req)
	if err != nil {
		ctrl.Log.Error("PaymentController.HandleAppointmentPayment error from usecase",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Duration(constvars.LoggingDurationKey, time.Since(start)),
			zap.Error(err),
		)
		if err == context.DeadlineExceeded {
			utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrServerDeadlineExceeded(err))
			return
		}
		utils.BuildErrorResponse(ctrl.Log, w, err)
		return
	}

	ctrl.Log.Info("PaymentController.HandleAppointmentPayment succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String("appointmentId", resp.AppointmentID),
	)
	utils.BuildSuccessResponse(w, constvars.StatusCreated, constvars.AppointmentPaymentSuccessMessage, resp)
}
