package controllers

import (
	"context"
	"encoding/json"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/utils"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

type JournalController struct {
	Log            *zap.Logger
	JournalUsecase contracts.JournalUsecase
}

func NewJournalController(logger *zap.Logger, journalUsecase contracts.JournalUsecase) *JournalController {
	return &JournalController{
		Log:            logger,
		JournalUsecase: journalUsecase,
	}
}

func (ctrl *JournalController) CreateJournal(w http.ResponseWriter, r *http.Request) {
	request := new(requests.CreateJournal)
	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrCannotParseJSON(err))
		return
	}
	request.SessionData = r.Context().Value(constvars.CONTEXT_SESSION_DATA_KEY).(string)

	err = utils.ValidateStruct(request)
	if err != nil {
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrInputValidation(err))
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	response, err := ctrl.JournalUsecase.CreateJournal(ctx, request)
	if err != nil {
		if err == context.DeadlineExceeded {
			utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrServerDeadlineExceeded(err))
			return
		}
		utils.BuildErrorResponse(ctrl.Log, w, err)
		return
	}

	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.CreateJournalSuccessMessage, response)
}

func (ctrl *JournalController) UpdateJournalByID(w http.ResponseWriter, r *http.Request) {
	request := new(requests.UpdateJournal)
	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrCannotParseJSON(err))
		return
	}
	request.SessionData = r.Context().Value(constvars.CONTEXT_SESSION_DATA_KEY).(string)
	request.JournalID = chi.URLParam(r, constvars.URLParamJournalID)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	response, err := ctrl.JournalUsecase.UpdateJournal(ctx, request)
	if err != nil {
		if err == context.DeadlineExceeded {
			utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrServerDeadlineExceeded(err))
			return
		}
		utils.BuildErrorResponse(ctrl.Log, w, err)
		return
	}

	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.UpdateJournalSuccessMessage, response)
}

func (ctrl *JournalController) FindJournalByID(w http.ResponseWriter, r *http.Request) {
	request := &requests.FindJournalByID{
		JournalID:   chi.URLParam(r, constvars.URLParamJournalID),
		SessionData: r.Context().Value(constvars.CONTEXT_SESSION_DATA_KEY).(string),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	response, err := ctrl.JournalUsecase.FindJournalByID(ctx, request)
	if err != nil {
		if err == context.DeadlineExceeded {
			utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrServerDeadlineExceeded(err))
			return
		}
		utils.BuildErrorResponse(ctrl.Log, w, err)
		return
	}

	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.FindJournalSuccessMessage, response)
}

func (ctrl *JournalController) DeleteJournalByID(w http.ResponseWriter, r *http.Request) {
	request := &requests.DeleteJournalByID{
		JournalID:   chi.URLParam(r, constvars.URLParamJournalID),
		SessionData: r.Context().Value(constvars.CONTEXT_SESSION_DATA_KEY).(string),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := ctrl.JournalUsecase.DeleteJournalByID(ctx, request)
	if err != nil {
		if err == context.DeadlineExceeded {
			utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrServerDeadlineExceeded(err))
			return
		}
		utils.BuildErrorResponse(ctrl.Log, w, err)
		return
	}

	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.DeleteJournalSuccessMessage, nil)
}
