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

type EducationLevelController struct {
	Log                   *zap.Logger
	EducationLevelUsecase contracts.EducationLevelUsecase
}

var (
	educationLevelControllerInstance *EducationLevelController
	onceEducationLevelController     sync.Once
)

func NewEducationLevelController(logger *zap.Logger, educationLevelUsecase contracts.EducationLevelUsecase) *EducationLevelController {
	onceEducationLevelController.Do(func() {
		instance := &EducationLevelController{
			Log:                   logger,
			EducationLevelUsecase: educationLevelUsecase,
		}
		educationLevelControllerInstance = instance
	})
	return educationLevelControllerInstance
}

func (ctrl *EducationLevelController) FindAll(w http.ResponseWriter, r *http.Request) {
	requestID, ok := r.Context().Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	if !ok || requestID == "" {
		ctrl.Log.Error("EducationLevelController.FindAll requestID not found in context")
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrMissingRequestID(nil))
		return
	}
	ctrl.Log.Info("EducationLevelController.FindAll called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	result, err := ctrl.EducationLevelUsecase.FindAll(ctx)
	if err != nil {
		ctrl.Log.Error("EducationLevelController.FindAll error from usecase",
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

	ctrl.Log.Info("EducationLevelController.FindAll succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Int(constvars.LoggingEducationLevelCountKey, len(result)),
	)
	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.GetEducationLevelSuccessMessage, result)
}
