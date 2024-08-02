package clinics

import (
	"context"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/utils"
	"net/http"
	"strconv"
	"time"

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

	result, paginationData, err := ctrl.ClinicUsecase.FindAll(ctx, page, pageSize)
	if err != nil {
		if err == context.DeadlineExceeded {
			utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrServerDeadlineExceeded(err))
			return
		}
		utils.BuildErrorResponse(ctrl.Log, w, err)
		return
	}

	utils.BuildSuccessResponseWithPagination(w, constvars.StatusOK, constvars.GetGenderSuccessMessage, paginationData, result)
}
