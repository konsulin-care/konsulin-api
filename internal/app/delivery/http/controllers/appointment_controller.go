package controllers

import (
	"context"
	"konsulin-service/internal/app/services/core/appointments"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/utils"
	"net/http"
	"time"

	"go.uber.org/zap"
)

type AppointmentController struct {
	Log                *zap.Logger
	AppointmentUsecase appointments.AppointmentUsecase
}

func NewAppointmentController(logger *zap.Logger, appointmentUsecase appointments.AppointmentUsecase) *AppointmentController {
	return &AppointmentController{
		Log:                logger,
		AppointmentUsecase: appointmentUsecase,
	}
}

func (ctrl *AppointmentController) FindAll(w http.ResponseWriter, r *http.Request) {
	sessionData := r.Context().Value("sessionData").(string)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	queryParamsRequest := utils.BuildQueryParamsRequest(r)

	response, err := ctrl.AppointmentUsecase.FindAll(ctx, sessionData, queryParamsRequest)
	if err != nil {
		if err == context.DeadlineExceeded {
			utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrServerDeadlineExceeded(err))
			return
		}
		utils.BuildErrorResponse(ctrl.Log, w, err)
		return
	}

	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.GetAppointmentSuccessMessage, response)
}
