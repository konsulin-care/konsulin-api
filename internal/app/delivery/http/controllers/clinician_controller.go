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

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

type ClinicianController struct {
	Log              *zap.Logger
	ClinicianUsecase contracts.ClinicianUsecase
}

var (
	clinicianControllerInstance *ClinicianController
	onceClinicianController     sync.Once
)

func NewClinicianController(logger *zap.Logger, clinicianUsecase contracts.ClinicianUsecase) *ClinicianController {
	onceClinicianController.Do(func() {
		instance := &ClinicianController{
			Log:              logger,
			ClinicianUsecase: clinicianUsecase,
		}
		clinicianControllerInstance = instance
	})
	return clinicianControllerInstance
}
func (ctrl *ClinicianController) CreatePracticeInformation(w http.ResponseWriter, r *http.Request) {
	requestID, ok := r.Context().Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	if !ok || requestID == "" {
		ctrl.Log.Error("ClinicianController.CreatePracticeInformation requestID not found in context")
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrMissingRequestID(nil))
		return
	}
	ctrl.Log.Info("ClinicianController.CreatePracticeInformation called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	request := new(requests.CreatePracticeInformation)
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		ctrl.Log.Error("ClinicianController.CreatePracticeInformation error decoding JSON",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrCannotParseJSON(err))
		return
	}

	sessionData, ok := r.Context().Value(constvars.CONTEXT_SESSION_DATA_KEY).(string)
	if !ok || sessionData == "" {
		ctrl.Log.Error("ClinicianController.CreatePracticeInformation sessionData not found in context",
			zap.String(constvars.LoggingRequestIDKey, requestID),
		)
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrMissingSessionData(nil))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	response, err := ctrl.ClinicianUsecase.CreatePracticeInformation(ctx, sessionData, request)
	if err != nil {
		ctrl.Log.Error("ClinicianController.CreatePracticeInformation error from usecase",
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

	ctrl.Log.Info("ClinicianController.CreatePracticeInformation succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)
	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.CreateClinicianClinicsSuccessMessage, response)
}

func (ctrl *ClinicianController) FindClinicsByClinicianID(w http.ResponseWriter, r *http.Request) {
	requestID, ok := r.Context().Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	if !ok || requestID == "" {
		ctrl.Log.Error("ClinicianController.FindClinicsByClinicianID requestID not found in context")
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrMissingRequestID(nil))
		return
	}
	ctrl.Log.Info("ClinicianController.FindClinicsByClinicianID called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	request := &requests.FindClinicianByClinicianID{
		PractitionerID:   chi.URLParam(r, constvars.URLParamClinicianID),
		OrganizationName: r.URL.Query().Get("name"),
	}
	ctrl.Log.Info("ClinicianController.FindClinicsByClinicianID parameters",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	result, err := ctrl.ClinicianUsecase.FindClinicsByClinicianID(ctx, request)
	if err != nil {
		ctrl.Log.Error("ClinicianController.FindClinicsByClinicianID error from usecase",
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

	ctrl.Log.Info("ClinicianController.FindClinicsByClinicianID succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)
	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.GetClinicianSummarySuccessfully, result)
}

func (ctrl *ClinicianController) FindAvailability(w http.ResponseWriter, r *http.Request) {
	requestID, ok := r.Context().Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	if !ok || requestID == "" {
		ctrl.Log.Error("ClinicianController.FindAvailability requestID not found in context")
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrMissingRequestID(nil))
		return
	}
	ctrl.Log.Info("ClinicianController.FindAvailability called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	request := &requests.FindAvailability{
		Year:               r.URL.Query().Get(constvars.URLQueryYear),
		Month:              r.URL.Query().Get(constvars.URLQueryMonth),
		PractitionerRoleID: r.URL.Query().Get(constvars.URLQueryParamPractitionerRoleID),
	}

	ctrl.Log.Info("ClinicianController.FindAvailability query parameters",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Any(constvars.LoggingRequestKey, request),
	)

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	result, err := ctrl.ClinicianUsecase.FindAvailability(ctx, request)
	if err != nil {
		ctrl.Log.Error("ClinicianController.FindAvailability error from usecase",
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

	ctrl.Log.Info("ClinicianController.FindAvailability succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)
	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.GetClinicsSuccessfully, result)
}

func (ctrl *ClinicianController) CreatePracticeAvailability(w http.ResponseWriter, r *http.Request) {
	requestID, ok := r.Context().Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	if !ok || requestID == "" {
		ctrl.Log.Error("ClinicianController.CreatePracticeAvailability requestID not found in context")
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrMissingRequestID(nil))
		return
	}
	ctrl.Log.Info("ClinicianController.CreatePracticeAvailability called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	request := new(requests.CreatePracticeAvailability)
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		ctrl.Log.Error("ClinicianController.CreatePracticeAvailability error decoding JSON",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrCannotParseJSON(err))
		return
	}

	sessionData, ok := r.Context().Value(constvars.CONTEXT_SESSION_DATA_KEY).(string)
	if !ok || sessionData == "" {
		ctrl.Log.Error("ClinicianController.CreatePracticeAvailability sessionData not found in context",
			zap.String(constvars.LoggingRequestIDKey, requestID),
		)
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrMissingSessionData(nil))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	response, err := ctrl.ClinicianUsecase.CreatePracticeAvailability(ctx, sessionData, request)
	if err != nil {
		ctrl.Log.Error("ClinicianController.CreatePracticeAvailability error from usecase",
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

	ctrl.Log.Info("ClinicianController.CreatePracticeAvailability succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Any("response", response),
	)
	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.CreateClinicianPracticeAvailabilitySuccessMessage, response)
}

func (ctrl *ClinicianController) DeleteClinicByID(w http.ResponseWriter, r *http.Request) {
	requestID, ok := r.Context().Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	if !ok || requestID == "" {
		ctrl.Log.Error("ClinicianController.DeleteClinicByID requestID not found in context")
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrMissingRequestID(nil))
		return
	}
	clinicID := chi.URLParam(r, constvars.URLParamClinicID)
	ctrl.Log.Info("ClinicianController.DeleteClinicByID called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingClinicIDKey, clinicID),
	)

	sessionData, ok := r.Context().Value(constvars.CONTEXT_SESSION_DATA_KEY).(string)
	if !ok || sessionData == "" {
		ctrl.Log.Error("ClinicianController.DeleteClinicByID sessionData not found in context",
			zap.String(constvars.LoggingRequestIDKey, requestID),
		)
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrMissingSessionData(nil))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	err := ctrl.ClinicianUsecase.DeleteClinicByID(ctx, sessionData, clinicID)
	if err != nil {
		ctrl.Log.Error("ClinicianController.DeleteClinicByID error from usecase",
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

	ctrl.Log.Info("ClinicianController.DeleteClinicByID succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingClinicIDKey, clinicID),
	)
	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.DeleteClinicianClinicSuccessMessage, nil)
}
