package controllers

import (
	"context"
	"net/http"
	"sync"
	"time"

	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/utils"

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
	if !decodeJSONBody(ctrl.Log, w, r, requestID, &req, "PaymentController.CreatePay error decoding JSON", false) {
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
	if !decodeJSONBody(ctrl.Log, w, r, requestID, &req, "PaymentController.HandleAppointmentPayment error decoding JSON", false) {
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
		zap.String("slotId", resp.SlotID),
	)
	utils.BuildSuccessResponse(w, constvars.StatusCreated, resp.Message, resp)
}
