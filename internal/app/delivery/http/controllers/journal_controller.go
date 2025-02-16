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

type JournalController struct {
	Log            *zap.Logger
	JournalUsecase contracts.JournalUsecase
}

var (
	journalControllerInstance *JournalController
	onceJournalController     sync.Once
)

func NewJournalController(logger *zap.Logger, journalUsecase contracts.JournalUsecase) *JournalController {
	onceJournalController.Do(func() {
		instance := &JournalController{
			Log:            logger,
			JournalUsecase: journalUsecase,
		}
		journalControllerInstance = instance
	})
	return journalControllerInstance
}

func (ctrl *JournalController) CreateJournal(w http.ResponseWriter, r *http.Request) {
	requestID, ok := r.Context().Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	if !ok || requestID == "" {
		ctrl.Log.Error("JournalController.CreateJournal requestID not found in context")
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrMissingRequestID(nil))
		return
	}
	ctrl.Log.Info("JournalController.CreateJournal called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	request := new(requests.CreateJournal)
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		ctrl.Log.Error("JournalController.CreateJournal error decoding JSON",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrCannotParseJSON(err))
		return
	}

	sessionData, ok := r.Context().Value(constvars.CONTEXT_SESSION_DATA_KEY).(string)
	if !ok || sessionData == "" {
		ctrl.Log.Error("JournalController.CreateJournal sessionData not found in context",
			zap.String(constvars.LoggingRequestIDKey, requestID),
		)
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrMissingSessionData(nil))
		return
	}
	request.SessionData = sessionData

	if err := utils.ValidateStruct(request); err != nil {
		ctrl.Log.Error("JournalController.CreateJournal validation error",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrInputValidation(err))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	response, err := ctrl.JournalUsecase.CreateJournal(ctx, request)
	if err != nil {
		ctrl.Log.Error("JournalController.CreateJournal error from usecase",
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

	ctrl.Log.Info("JournalController.CreateJournal succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)
	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.CreateJournalSuccessMessage, response)
}

func (ctrl *JournalController) UpdateJournalByID(w http.ResponseWriter, r *http.Request) {
	requestID, ok := r.Context().Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	if !ok || requestID == "" {
		ctrl.Log.Error("JournalController.UpdateJournalByID requestID not found in context")
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrMissingRequestID(nil))
		return
	}
	ctrl.Log.Info("JournalController.UpdateJournalByID called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	request := new(requests.UpdateJournal)
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		ctrl.Log.Error("JournalController.UpdateJournalByID error decoding JSON",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrCannotParseJSON(err))
		return
	}

	sessionData, ok := r.Context().Value(constvars.CONTEXT_SESSION_DATA_KEY).(string)
	if !ok || sessionData == "" {
		ctrl.Log.Error("JournalController.UpdateJournalByID sessionData not found in context",
			zap.String(constvars.LoggingRequestIDKey, requestID),
		)
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrMissingSessionData(nil))
		return
	}
	request.SessionData = sessionData
	request.JournalID = chi.URLParam(r, constvars.URLParamJournalID)

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	response, err := ctrl.JournalUsecase.UpdateJournal(ctx, request)
	if err != nil {
		ctrl.Log.Error("JournalController.UpdateJournalByID error from usecase",
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

	ctrl.Log.Info("JournalController.UpdateJournalByID succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)
	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.UpdateJournalSuccessMessage, response)
}

func (ctrl *JournalController) FindJournalByID(w http.ResponseWriter, r *http.Request) {
	requestID, ok := r.Context().Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	if !ok || requestID == "" {
		ctrl.Log.Error("JournalController.FindJournalByID requestID not found in context")
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrMissingRequestID(nil))
		return
	}
	ctrl.Log.Info("JournalController.FindJournalByID called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	request := &requests.FindJournalByID{
		JournalID:   chi.URLParam(r, constvars.URLParamJournalID),
		SessionData: r.Context().Value(constvars.CONTEXT_SESSION_DATA_KEY).(string),
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	response, err := ctrl.JournalUsecase.FindJournalByID(ctx, request)
	if err != nil {
		ctrl.Log.Error("JournalController.FindJournalByID error from usecase",
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

	ctrl.Log.Info("JournalController.FindJournalByID succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingJournalIDKey, request.JournalID),
	)
	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.FindJournalSuccessMessage, response)
}

func (ctrl *JournalController) DeleteJournalByID(w http.ResponseWriter, r *http.Request) {
	requestID, ok := r.Context().Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	if !ok || requestID == "" {
		ctrl.Log.Error("JournalController.DeleteJournalByID requestID not found in context")
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrMissingRequestID(nil))
		return
	}
	ctrl.Log.Info("JournalController.DeleteJournalByID called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	request := &requests.DeleteJournalByID{
		JournalID:   chi.URLParam(r, constvars.URLParamJournalID),
		SessionData: r.Context().Value(constvars.CONTEXT_SESSION_DATA_KEY).(string),
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	err := ctrl.JournalUsecase.DeleteJournalByID(ctx, request)
	if err != nil {
		ctrl.Log.Error("JournalController.DeleteJournalByID error from usecase",
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

	ctrl.Log.Info("JournalController.DeleteJournalByID succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingJournalIDKey, request.JournalID),
	)
	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.DeleteJournalSuccessMessage, nil)
}
