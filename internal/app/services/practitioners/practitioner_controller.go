package practitioners

import (
	"context"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/utils"
	"net/http"
	"time"

	"github.com/goccy/go-json"
)

type PractitionerController struct {
	PractitionerUsecase PractitionerUsecase
}

func NewPractitionerController(PractitionerUsecase PractitionerUsecase) *PractitionerController {
	return &PractitionerController{
		PractitionerUsecase: PractitionerUsecase,
	}
}

func (ctrl *PractitionerController) GetPractitionerProfileBySession(w http.ResponseWriter, r *http.Request) {
	// Get session data from context
	sessionData := r.Context().Value("sessionData").(string)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := ctrl.PractitionerUsecase.GetPractitionerProfileBySession(ctx, sessionData)
	if err != nil {
		if err == context.DeadlineExceeded {
			utils.BuildErrorResponse(w, exceptions.ErrServerDeadlineExceeded(err))
			return
		}
		utils.BuildErrorResponse(w, err)
		return
	}

	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.ProfileGetSuccess, result)
}

func (ctrl *PractitionerController) UpdateProfileBySession(w http.ResponseWriter, r *http.Request) {
	request := new(requests.UpdateProfile)
	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		utils.BuildErrorResponse(w, exceptions.ErrCannotParseJSON(err))
		return
	}

	// Validate request
	err = utils.ValidateStruct(request)
	if err != nil {
		utils.BuildErrorResponse(w, exceptions.ErrInputValidation(err))
		return
	}

	sessionData := r.Context().Value("sessionData").(string)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	response, err := ctrl.PractitionerUsecase.UpdatePractitionerProfileBySession(ctx, sessionData, request)
	if err != nil {
		if err == context.DeadlineExceeded {
			utils.BuildErrorResponse(w, exceptions.ErrServerDeadlineExceeded(err))
			return
		}
		utils.BuildErrorResponse(w, err)
		return
	}

	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.UserUpdatedSuccess, response)
}
