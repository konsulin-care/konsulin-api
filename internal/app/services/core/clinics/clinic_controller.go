package clinics

import (
	"context"
	"konsulin-service/internal/pkg/constvars"
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
	ClinicUsecase ClinicUsecase
}

func NewClinicController(logger *zap.Logger, clinicUsecase ClinicUsecase) *ClinicController {
	return &ClinicController{
		Log:           logger,
		ClinicUsecase: clinicUsecase,
	}
}

func (ctrl *ClinicController) FindAll(w http.ResponseWriter, r *http.Request) {
	pageStr := r.URL.Query().Get("page")
	pageSizeStr := r.URL.Query().Get("page_size")
	nameStr := r.URL.Query().Get("name")
	fetchType := r.URL.Query().Get("type")

	page, err := strconv.Atoi(pageStr)
	if err != nil || page <= 0 {
		page = 1
	}

	pageSize, err := strconv.Atoi(pageSizeStr)
	if err != nil || pageSize <= 0 {
		pageSize = 10
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

	// err := utils.ValidateUrlParamID(clinicianID)
	// if err != nil {
	// 	utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrURLParamIDValidation(err, constvars.URLParamPractitionerID))
	// 	return
	// }

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
	pageStr := r.URL.Query().Get("page")
	pageSizeStr := r.URL.Query().Get("page_size")
	nameStr := r.URL.Query().Get("name")

	page, err := strconv.Atoi(pageStr)
	if err != nil || page <= 0 {
		page = 1
	}

	pageSize, err := strconv.Atoi(pageSizeStr)
	if err != nil || pageSize <= 0 {
		pageSize = 10
	}

	clinicID := chi.URLParam(r, constvars.URLParamClinicID)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	result, paginationData, err := ctrl.ClinicUsecase.FindAllCliniciansByClinicID(ctx, nameStr, clinicID, page, pageSize)
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
