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
	requestID, ok := r.Context().Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	if !ok {
		ctrl.Log.Error("AppointmentController.FindAll requestID not found in context")
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrMissingRequestID(nil))
		return
	}

	sessionData, ok := r.Context().Value(constvars.CONTEXT_SESSION_DATA_KEY).(string)
	if !ok {
		ctrl.Log.Error("AppointmentController.FindAll sessionData not found in context",
			zap.String(constvars.LoggingRequestIDKey, requestID))

		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrMissingSessionData(nil))
		return
	}

	ctrl.Log.Info("AppointmentController.FindAll called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingSessionDataKey, sessionData))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	queryParamsRequest := utils.BuildQueryParamsRequest(r)
	ctrl.Log.Info("AppointmentController.FindAll query parameters",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Any(constvars.LoggingQueryParamsKey, queryParamsRequest))

	response, err := ctrl.AppointmentUsecase.FindAll(ctx, sessionData, queryParamsRequest)
	if err != nil {
		ctrl.Log.Error("Error in AppointmentUsecase.FindAll",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err))

		if err == context.DeadlineExceeded {
			utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrServerDeadlineExceeded(err))
			return
		}
		utils.BuildErrorResponse(ctrl.Log, w, err)
		return
	}

	ctrl.Log.Info("AppointmentController.FindAll succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Int(constvars.LoggingResponseLengthKey, len(response)))
	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.GetAppointmentSuccessMessage, response)
}

func (ctrl *AppointmentController) UpcomingAppointment(w http.ResponseWriter, r *http.Request) {
	requestID, ok := r.Context().Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	if !ok {
		ctrl.Log.Error("AppointmentController.UpcomingAppointment requestID not found in context")
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrMissingRequestID(nil))
		return
	}

	sessionData, ok := r.Context().Value(constvars.CONTEXT_SESSION_DATA_KEY).(string)
	if !ok {
		ctrl.Log.Error("AppointmentController.UpcomingAppointment sessionData not found in context",
			zap.String(constvars.LoggingRequestIDKey, requestID))

		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrMissingSessionData(nil))
		return
	}
	ctrl.Log.Info("AppointmentController.UpcomingAppointment called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingSessionDataKey, sessionData))

	// Create a context with timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Build query parameters.
	queryParamsRequest := utils.BuildQueryParamsRequest(r)
	ctrl.Log.Info("AppointmentController.UpcomingAppointment query parameters",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Any(constvars.LoggingQueryParamsKey, queryParamsRequest))

	// Call the usecase.
	response, err := ctrl.AppointmentUsecase.FindUpcomingAppointment(ctx, sessionData, queryParamsRequest)
	if err != nil {
		ctrl.Log.Error("AppointmentController.UpcomingAppointment AppointmentUsecase.FindUpcomingAppointment error",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err))

		if err == context.DeadlineExceeded {
			utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrServerDeadlineExceeded(err))
			return
		}
		utils.BuildErrorResponse(ctrl.Log, w, err)
		return
	}

	ctrl.Log.Info("AppointmentController.UpcomingAppointment succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Any(constvars.LoggingResponseKey, response))

	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.GetAppointmentSuccessMessage, response)
}

func (ctrl *AppointmentController) CreateAppointment(w http.ResponseWriter, r *http.Request) {
	requestID, ok := r.Context().Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	if !ok {
		ctrl.Log.Error("AppointmentController.CreateAppointment requestID not found in context")
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrMissingRequestID(nil))
		return
	}

	sessionData, ok := r.Context().Value(constvars.CONTEXT_SESSION_DATA_KEY).(string)
	if !ok {
		ctrl.Log.Error("AppointmentController.CreateAppointment sessionData not found in context",
			zap.String(constvars.LoggingRequestIDKey, requestID))

		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrMissingSessionData(nil))
		return
	}
	ctrl.Log.Info("AppointmentController.CreateAppointment called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingSessionDataKey, sessionData))

	// Bind body to request
	request := new(requests.CreateAppointmentRequest)
	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		ctrl.Log.Error("AppointmentController.CreateAppointment Failed to decode JSON request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err))

		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrCannotParseJSON(err))
		return
	}

	ctrl.Log.Info("AppointmentController.CreateAppointment request decoded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Any(constvars.LoggingRequestKey, request))

	err = utils.ValidateStruct(request)
	if err != nil {
		ctrl.Log.Error("AppointmentController.CreateAppointment Validation error",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err))

		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrInputValidation(err))
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	response, err := ctrl.AppointmentUsecase.CreateAppointment(ctx, sessionData, request)
	if err != nil {
		ctrl.Log.Error("AppointmentController.CreateAppointment AppointmentUsecase.CreateAppointment error",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err))

		if err == context.DeadlineExceeded {
			utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrServerDeadlineExceeded(err))
			return
		}
		utils.BuildErrorResponse(ctrl.Log, w, err)
		return
	}

	ctrl.Log.Info("AppointmentController.CreateAppointment succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Any(constvars.LoggingResponseKey, response))

	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.CreatePatientAppointmentSuccessMessage, response)
}
