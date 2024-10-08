package controllers

import (
	"context"
	educationLevels "konsulin-service/internal/app/services/core/education_levels"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/utils"
	"net/http"
	"time"

	"go.uber.org/zap"
)

type EducationLevelController struct {
	Log                   *zap.Logger
	EducationLevelUsecase educationLevels.EducationLevelUsecase
}

func NewEducationLevelController(logger *zap.Logger, educationLevelUsecase educationLevels.EducationLevelUsecase) *EducationLevelController {
	return &EducationLevelController{
		Log:                   logger,
		EducationLevelUsecase: educationLevelUsecase,
	}
}

func (ctrl *EducationLevelController) FindAll(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := ctrl.EducationLevelUsecase.FindAll(ctx)
	if err != nil {
		if err == context.DeadlineExceeded {
			utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrServerDeadlineExceeded(err))
			return
		}
		utils.BuildErrorResponse(ctrl.Log, w, err)
		return
	}

	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.GetEducationLevelSuccessMessage, result)
}
