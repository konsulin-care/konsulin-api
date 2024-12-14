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
	"time"

	"go.uber.org/zap"
)

type PaymentController struct {
	Log            *zap.Logger
	PaymentUsecase contracts.PaymentUsecase
}

func NewPaymentController(logger *zap.Logger, paymentUsecase contracts.PaymentUsecase) *PaymentController {
	return &PaymentController{
		Log:            logger,
		PaymentUsecase: paymentUsecase,
	}
}

func (ctrl *PaymentController) PaymentRoutingCallback(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	request := new(requests.PaymentRoutingCallback)
	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrCannotParseJSON(err))
		return
	}

	err = ctrl.PaymentUsecase.PaymentRoutingCallback(ctx, request)
	if err != nil {
		if err == context.DeadlineExceeded {
			utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrServerDeadlineExceeded(err))
			return
		}
		utils.BuildErrorResponse(ctrl.Log, w, err)
		return
	}

	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.PaymentRoutingCallbackSuccessfullyCalled, request.PaymentStatus)
}
