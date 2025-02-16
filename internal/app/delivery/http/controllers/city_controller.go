package controllers

import (
	"context"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/utils"
	"net/http"
	"sync"
	"time"

	"go.uber.org/zap"
)

type CityController struct {
	Log         *zap.Logger
	CityUsecase contracts.CityUsecase
}

var (
	cityControllerInstance *CityController
	onceCityController     sync.Once
)

func NewCityController(logger *zap.Logger, cityUsecase contracts.CityUsecase) *CityController {
	onceCityController.Do(func() {
		instance := &CityController{
			Log:         logger,
			CityUsecase: cityUsecase,
		}
		cityControllerInstance = instance
	})
	return cityControllerInstance
}
func (ctrl *CityController) FindAll(w http.ResponseWriter, r *http.Request) {
	requestID, ok := r.Context().Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	if !ok || requestID == "" {
		ctrl.Log.Error("CityController.FindAll error: requestID not found in context")
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrMissingRequestID(nil))
		return
	}
	ctrl.Log.Info("CityController.FindAll called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	queryParamsRequest := utils.BuildQueryParamsRequest(r)
	ctrl.Log.Info("CityController.FindAll query parameters",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Any(constvars.LoggingQueryParamsKey, queryParamsRequest),
	)

	response, err := ctrl.CityUsecase.FindAll(ctx, queryParamsRequest)
	if err != nil {
		ctrl.Log.Error("CityController.FindAll error from usecase",
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

	ctrl.Log.Info("CityController.FindAll succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Int(constvars.LoggingResponseCountKey, len(response)),
	)
	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.GetCitySuccessMessage, response)
}
