package controllers

import (
	"context"
	"encoding/json"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/utils"
	"net/http"
	"time"

	"go.uber.org/zap"
)

type PaymentController struct {
	Log *zap.Logger
}

func NewPaymentController(logger *zap.Logger) *PaymentController {
	return &PaymentController{
		Log: logger,
	}
}

func (ctrl *PaymentController) PaymentRoutingCallback(w http.ResponseWriter, r *http.Request) {
	_, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	request := new(requests.Transaction)
	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrCannotParseJSON(err))
		return
	}

	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.GetAppointmentSuccessMessage, request.PaymentRouting[0].TrxStatus)
}
