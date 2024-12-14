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

type AppointmentController struct {
	Log                *zap.Logger
	AppointmentUsecase contracts.AppointmentUsecase
}

func NewAppointmentController(logger *zap.Logger, appointmentUsecase contracts.AppointmentUsecase) *AppointmentController {
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

func (ctrl *AppointmentController) CreateAppointment(w http.ResponseWriter, r *http.Request) {
	// Bind body to request
	request := new(requests.CreateAppointmentRequest)
	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrCannotParseJSON(err))
		return
	}

	err = utils.ValidateStruct(request)
	if err != nil {
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrInputValidation(err))
		return
	}

	// Get session data from context
	sessionData := r.Context().Value("sessionData").(string)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	response, err := ctrl.AppointmentUsecase.CreateAppointment(ctx, sessionData, request)
	if err != nil {
		if err == context.DeadlineExceeded {
			utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrServerDeadlineExceeded(err))
			return
		}
		utils.BuildErrorResponse(ctrl.Log, w, err)
		return
	}

	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.CreatePatientAppointmentSuccessMessage, response)
}
