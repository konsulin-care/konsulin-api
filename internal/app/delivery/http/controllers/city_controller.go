package controllers

import (
	"context"
	"konsulin-service/internal/app/services/core/cities"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/utils"
	"net/http"
	"time"

	"go.uber.org/zap"
)

type CityController struct {
	Log         *zap.Logger
	CityUsecase cities.CityUsecase
}

func NewCityController(logger *zap.Logger, cityUsecase cities.CityUsecase) *CityController {
	return &CityController{
		Log:         logger,
		CityUsecase: cityUsecase,
	}
}

func (ctrl *CityController) FindAll(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	queryParamsRequest := utils.BuildQueryParamsRequest(r)

	response, err := ctrl.CityUsecase.FindAll(ctx, queryParamsRequest)
	if err != nil {
		if err == context.DeadlineExceeded {
			utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrServerDeadlineExceeded(err))
			return
		}
		utils.BuildErrorResponse(ctrl.Log, w, err)
		return
	}

	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.GetCitySuccessMessage, response)
}
