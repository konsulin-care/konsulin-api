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

type GenderController struct {
	Log           *zap.Logger
	GenderUsecase contracts.GenderUsecase
}

var (
	genderControllerInstance *GenderController
	onceGenderController     sync.Once
)

func NewGenderController(logger *zap.Logger, genderUsecase contracts.GenderUsecase) *GenderController {
	onceGenderController.Do(func() {
		instance := &GenderController{
			Log:           logger,
			GenderUsecase: genderUsecase,
		}
		genderControllerInstance = instance
	})
	return genderControllerInstance
}

func (ctrl *GenderController) FindAll(w http.ResponseWriter, r *http.Request) {
	requestID, ok := r.Context().Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	if !ok || requestID == "" {
		ctrl.Log.Error("GenderController.FindAll requestID not found in context")
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrMissingRequestID(nil))
		return
	}
	ctrl.Log.Info("GenderController.FindAll called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	result, err := ctrl.GenderUsecase.FindAll(ctx)
	if err != nil {
		ctrl.Log.Error("GenderController.FindAll error from usecase",
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

	ctrl.Log.Info("GenderController.FindAll succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Int(constvars.LoggingGenderCountKey, len(result)),
	)
	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.GetGenderSuccessMessage, result)
}
