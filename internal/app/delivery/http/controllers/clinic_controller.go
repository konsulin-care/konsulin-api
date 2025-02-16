package controllers

import (
	"context"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/utils"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

type ClinicController struct {
	Log           *zap.Logger
	ClinicUsecase contracts.ClinicUsecase
}

var (
	clinicControllerInstance *ClinicController
	onceClinicController     sync.Once
)

func NewClinicController(logger *zap.Logger, clinicUsecase contracts.ClinicUsecase) *ClinicController {
	onceClinicController.Do(func() {
		instance := &ClinicController{
			Log:           logger,
			ClinicUsecase: clinicUsecase,
		}
		clinicControllerInstance = instance
	})
	return clinicControllerInstance
}

func (ctrl *ClinicController) FindAll(w http.ResponseWriter, r *http.Request) {
	requestID, ok := r.Context().Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	if !ok || requestID == "" {
		ctrl.Log.Error("ClinicController.FindAll error: requestID not found in context")
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrMissingRequestID(nil))
		return
	}
	ctrl.Log.Info("ClinicController.FindAll called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	nameStr := r.URL.Query().Get(constvars.URLQueryParamName)
	fetchType := r.URL.Query().Get(constvars.URLQueryParamType)
	var page, pageSize int

	if fetchType == constvars.FhirFetchResourceTypePaged {
		pageStr := r.URL.Query().Get(constvars.URLQueryParamPage)
		pageSizeStr := r.URL.Query().Get(constvars.URLQueryParamPageSize)

		pageInt, err := strconv.Atoi(pageStr)
		if err != nil || pageInt <= 0 {
			page = 1
		} else {
			page = pageInt
		}

		pageSizeInt, err := strconv.Atoi(pageSizeStr)
		if err != nil || pageSizeInt <= 0 {
			pageSize = 10
		} else {
			pageSize = pageSizeInt
		}
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	ctrl.Log.Info("ClinicController.FindAll building usecase call",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	result, paginationData, err := ctrl.ClinicUsecase.FindAll(ctx, nameStr, fetchType, page, pageSize)
	if err != nil {
		ctrl.Log.Error("ClinicController.FindAll error from usecase",
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

	ctrl.Log.Info("ClinicController.FindAll succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Int(constvars.LoggingClinicCountKey, len(result)),
	)
	utils.BuildSuccessResponseWithPagination(w, constvars.StatusOK, constvars.GetClinicsSuccessfully, paginationData, result)
}

func (ctrl *ClinicController) FindClinicianByClinicAndClinicianID(w http.ResponseWriter, r *http.Request) {
	requestID, ok := r.Context().Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	if !ok || requestID == "" {
		ctrl.Log.Error("ClinicController.FindClinicianByClinicAndClinicianID error: requestID not found in context")
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrMissingRequestID(nil))
		return
	}

	clinicID := chi.URLParam(r, constvars.URLParamClinicID)
	clinicianID := chi.URLParam(r, constvars.URLParamClinicianID)
	ctrl.Log.Info("ClinicController.FindClinicianByClinicAndClinicianID called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingClinicIDKey, clinicID),
		zap.String(constvars.LoggingClinicianIDKey, clinicianID),
	)

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	result, err := ctrl.ClinicUsecase.FindClinicianByClinicAndClinicianID(ctx, clinicID, clinicianID)
	if err != nil {
		ctrl.Log.Error("ClinicController.FindClinicianByClinicAndClinicianID error from usecase",
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

	ctrl.Log.Info("ClinicController.FindClinicianByClinicAndClinicianID succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)
	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.GetClinicianSummarySuccessfully, result)
}

func (ctrl *ClinicController) FindAllCliniciansByClinicID(w http.ResponseWriter, r *http.Request) {
	requestID, ok := r.Context().Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	if !ok || requestID == "" {
		ctrl.Log.Error("ClinicController.FindAllCliniciansByClinicID error: requestID not found in context")
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrMissingRequestID(nil))
		return
	}
	ctrl.Log.Info("ClinicController.FindAllCliniciansByClinicID called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	request := &requests.FindAllCliniciansByClinicID{
		PractitionerName: r.URL.Query().Get("name"),
		City:             r.URL.Query().Get("city"),
		Days:             r.URL.Query().Get("days"),
		StartTime:        r.URL.Query().Get("start_time"),
		EndTime:          r.URL.Query().Get("end_time"),
		ClinicID:         chi.URLParam(r, constvars.URLParamClinicID),
	}

	if err := utils.ValidateUrlParamID(request.ClinicID); err != nil {
		ctrl.Log.Error("ClinicController.FindAllCliniciansByClinicID URL parameter validation error",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrURLParamIDValidation(err, constvars.URLParamClinicID))
		return
	}

	if request.StartTime == "" {
		request.StartTime = constvars.DEFAULT_CLINICIAN_PRACTICE_START_TIME_PARAMS
	}
	if request.EndTime == "" {
		request.EndTime = constvars.DEFAULT_CLINICIAN_PRACTICE_END_TIME_PARAMS
	}
	if request.Days == "" {
		request.Days = constvars.DEFAULT_CLINICIAN_DESIRED_DAYS_PARAMS
	}

	pageStr := r.URL.Query().Get(constvars.URLQueryParamPage)
	pageSizeStr := r.URL.Query().Get(constvars.URLQueryParamPageSize)

	request.Page, _ = strconv.Atoi(pageStr)
	if request.Page <= 0 {
		request.Page = 1
	}

	request.Page, _ = strconv.Atoi(pageSizeStr)
	if request.Page <= 0 {
		request.Page = 10
	}

	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()

	result, paginationData, err := ctrl.ClinicUsecase.FindAllCliniciansByClinicID(ctx, request)
	if err != nil {
		ctrl.Log.Error("ClinicController.FindAllCliniciansByClinicID error from usecase",
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

	ctrl.Log.Info("ClinicController.FindAllCliniciansByClinicID succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Int(constvars.LoggingClinicianCountKey, len(result)),
	)
	utils.BuildSuccessResponseWithPagination(w, constvars.StatusOK, constvars.GetCliniciansSuccessfully, paginationData, result)
}

func (ctrl *ClinicController) FindByID(w http.ResponseWriter, r *http.Request) {
	clinicID := chi.URLParam(r, constvars.URLParamClinicID)
	if err := utils.ValidateUrlParamID(clinicID); err != nil {
		ctrl.Log.Error("ClinicController.FindByID URL parameter validation error",
			zap.String(constvars.LoggingClinicIDKey, clinicID),
			zap.Error(err),
		)
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrURLParamIDValidation(err, constvars.URLParamClinicID))
		return
	}

	requestID, ok := r.Context().Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	if !ok || requestID == "" {
		ctrl.Log.Error("ClinicController.FindByID error: requestID not found in context")
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrMissingRequestID(nil))
		return
	}
	ctrl.Log.Info("ClinicController.FindByID called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingClinicIDKey, clinicID),
	)

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	result, err := ctrl.ClinicUsecase.FindByID(ctx, clinicID)
	if err != nil {
		ctrl.Log.Error("ClinicController.FindByID error from usecase",
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

	ctrl.Log.Info("ClinicController.FindByID succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingClinicIDKey, clinicID),
	)
	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.GetClinicsSuccessfully, result)
}
