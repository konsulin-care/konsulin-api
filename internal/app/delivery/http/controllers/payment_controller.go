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
	requestID, ok := r.Context().Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	if !ok || requestID == "" {
		ctrl.Log.Error("PaymentController.PaymentRoutingCallback requestID not found in context")
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrMissingRequestID(nil))
		return
	}
	ctrl.Log.Info("PaymentController.PaymentRoutingCallback called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	request := new(requests.PaymentRoutingCallback)
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		ctrl.Log.Error("PaymentController.PaymentRoutingCallback error decoding JSON",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrCannotParseJSON(err))
		return
	}
	ctrl.Log.Info("PaymentController.PaymentRoutingCallback request decoded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	err := ctrl.PaymentUsecase.PaymentRoutingCallback(ctx, request)
	if err != nil {
		ctrl.Log.Error("PaymentController.PaymentRoutingCallback error from usecase",
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

	ctrl.Log.Info("PaymentController.PaymentRoutingCallback succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)
	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.PaymentRoutingCallbackSuccessfullyCalled, request.PaymentStatus)
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
