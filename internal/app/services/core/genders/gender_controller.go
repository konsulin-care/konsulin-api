package genders

import (
	"context"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/utils"
	"net/http"
	"time"

	"go.uber.org/zap"
)

type GenderController struct {
	Log           *zap.Logger
	GenderUsecase GenderUsecase
}

func NewGenderController(logger *zap.Logger, genderUsecase GenderUsecase) *GenderController {
	return &GenderController{
		Log:           logger,
		GenderUsecase: genderUsecase,
	}
}

func (ctrl *GenderController) FindAll(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := ctrl.GenderUsecase.FindAll(ctx)
	if err != nil {
		if err == context.DeadlineExceeded {
			utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrServerDeadlineExceeded(err))
			return
		}
		utils.BuildErrorResponse(ctrl.Log, w, err)
		return
	}

	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.GetGenderSuccessMessage, result)
}
