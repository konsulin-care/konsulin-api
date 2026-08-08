package controllers

type AuthController struct {
	AuthUsecase contracts.AuthUsecase
}

func (ctrl *AuthController) CreateMagicLink(w http.ResponseWriter, r *http.Request) {
	request := new(requests.SupertokenPasswordlessCreateMagicLink)
	json.NewDecoder(r.Body).Decode(&request)

	userExistsOutput, err := ctrl.AuthUsecase.CheckUserExists(ctx, request.Email)
	if err != nil {
		utils.BuildErrorResponse(ctrl.Log, w, err)
		return
	}

	err = ctrl.AuthUsecase.CreateMagicLink(ctx, request)
	if err != nil {
		utils.BuildErrorResponse(ctrl.Log, w, err)
		return
	}

	utils.BuildSuccessResponse(w, constvars.StatusOK, "magic link created", nil)
}

func (ctrl *AuthController) CreateAnonymousSession(w http.ResponseWriter, r *http.Request) {
	result, err := ctrl.AuthUsecase.CreateAnonymousSession(ctx, existingToken, forceNew)
	if err != nil {
		utils.BuildErrorResponse(ctrl.Log, w, err)
		return
	}

	response := map[string]interface{}{
		"token":    result.Token,
		"guest_id": result.GuestID,
		"is_new":   result.IsNew,
		"role":     "guest",
	}
	utils.BuildSuccessResponse(w, constvars.StatusOK, "Anonymous session created", response)
}

func (ctrl *AuthController) PasswordlessEmailExists(w http.ResponseWriter, r *http.Request) {
	output, err := ctrl.AuthUsecase.CheckUserExists(ctx, r.URL.Query().Get("email"))
	if err != nil {
		utils.BuildErrorResponse(ctrl.Log, w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}
