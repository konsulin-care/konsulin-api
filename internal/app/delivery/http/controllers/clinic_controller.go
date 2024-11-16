package controllers

import (
	"context"
	"konsulin-service/internal/app/services/core/clinics"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/utils"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

type ClinicController struct {
	Log           *zap.Logger
	ClinicUsecase clinics.ClinicUsecase
}

func NewClinicController(logger *zap.Logger, clinicUsecase clinics.ClinicUsecase) *ClinicController {
	return &ClinicController{
		Log:           logger,
		ClinicUsecase: clinicUsecase,
	}
}

func (ctrl *ClinicController) FindAll(w http.ResponseWriter, r *http.Request) {
	nameStr := r.URL.Query().Get("name")
	fetchType := r.URL.Query().Get("type")

	var (
		page     int
		pageSize int
	)

	if fetchType == constvars.FhirFetchResourceTypePaged {
		pageStr := r.URL.Query().Get("page")
		pageSizeStr := r.URL.Query().Get("page_size")

		pageInt, err := strconv.Atoi(pageStr)
		if err != nil || page <= 0 {
			page = 1
		}

		pageSizeInt, err := strconv.Atoi(pageSizeStr)
		if err != nil || pageSize <= 0 {
			pageSize = 10
		}

		page = pageInt
		pageSize = pageSizeInt
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, paginationData, err := ctrl.ClinicUsecase.FindAll(ctx, nameStr, fetchType, page, pageSize)
	if err != nil {
		if err == context.DeadlineExceeded {
			utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrServerDeadlineExceeded(err))
			return
		}
		utils.BuildErrorResponse(ctrl.Log, w, err)
		return
	}

	utils.BuildSuccessResponseWithPagination(w, constvars.StatusOK, constvars.GetClinicsSuccessfully, paginationData, result)
}

func (ctrl *ClinicController) FindClinicianByClinicAndClinicianID(w http.ResponseWriter, r *http.Request) {
	clinicID := chi.URLParam(r, constvars.URLParamClinicID)
	clinicianID := chi.URLParam(r, constvars.URLParamClinicianID)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := ctrl.ClinicUsecase.FindClinicianByClinicAndClinicianID(ctx, clinicID, clinicianID)
	if err != nil {
		if err == context.DeadlineExceeded {
			utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrServerDeadlineExceeded(err))
			return
		}
		utils.BuildErrorResponse(ctrl.Log, w, err)
		return
	}

	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.GetClinicianSummarySuccessfully, result)
}

func (ctrl *ClinicController) FindAllCliniciansByClinicID(w http.ResponseWriter, r *http.Request) {
	request := &requests.FindAllCliniciansByClinicID{
		PractitionerName: r.URL.Query().Get("name"),
		City:             r.URL.Query().Get("city"),
		Days:             r.URL.Query().Get("days"),
		StartTime:        r.URL.Query().Get("start_time"),
		EndTime:          r.URL.Query().Get("end_time"),
		ClinicID:         chi.URLParam(r, constvars.URLParamClinicID),
	}

	err := utils.ValidateUrlParamID(request.ClinicID)
	if err != nil {
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

	pageStr := r.URL.Query().Get("page")
	pageSizeStr := r.URL.Query().Get("page_size")

	request.Page, err = strconv.Atoi(pageStr)
	if err != nil || request.Page <= 0 {
		request.Page = 1
	}

	request.Page, err = strconv.Atoi(pageSizeStr)
	if err != nil || request.Page <= 0 {
		request.Page = 10
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	result, paginationData, err := ctrl.ClinicUsecase.FindAllCliniciansByClinicID(ctx, request)
	if err != nil {
		if err == context.DeadlineExceeded {
			utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrServerDeadlineExceeded(err))
			return
		}
		utils.BuildErrorResponse(ctrl.Log, w, err)
		return
	}

	utils.BuildSuccessResponseWithPagination(w, constvars.StatusOK, constvars.GetCliniciansSuccessfully, paginationData, result)
}

func (ctrl *ClinicController) FindByID(w http.ResponseWriter, r *http.Request) {
	clinicID := chi.URLParam(r, constvars.URLParamClinicID)

	err := utils.ValidateUrlParamID(clinicID)
	if err != nil {
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrURLParamIDValidation(err, constvars.URLParamClinicID))
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := ctrl.ClinicUsecase.FindByID(ctx, clinicID)
	if err != nil {
		if err == context.DeadlineExceeded {
			utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrServerDeadlineExceeded(err))
			return
		}
		utils.BuildErrorResponse(ctrl.Log, w, err)
		return
	}

	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.GetClinicsSuccessfully, result)
}
