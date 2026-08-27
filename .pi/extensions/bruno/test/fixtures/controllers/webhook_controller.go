package controllers

type WebhookController struct {
	Usecase webhook.Usecase
	amqp    rabbit.Client
}

func (ctrl *WebhookController) HandleEnqueueWebHook(w http.ResponseWriter, r *http.Request) {
	out, err := ctrl.Usecase.Enqueue(ctx, &webhook.EnqueueInput{
		ServiceName: serviceName,
		Method:      "POST",
		RawJSON:     raw,
	})
	if err != nil {
		utils.BuildErrorResponse(ctrl.Log, w, err)
		return
	}
	utils.BuildSuccessResponse(w, http.StatusAccepted, "enqueued", map[string]interface{}{
		"request_id": out.RequestID,
		"status":     "pending",
	})
}

func (ctrl *WebhookController) HandleAsyncServiceResultCallback(w http.ResponseWriter, r *http.Request) {
	err := ctrl.Usecase.HandleAsyncServiceResult(r.Context(), &webhook.HandleAsyncServiceResultInput{
		ServiceRequestID: req.ServiceRequestID,
		Result:           req.Result,
	})
	if err != nil {
		utils.BuildErrorResponse(ctrl.Log, w, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (ctrl *WebhookController) HandleGetAsyncServiceResult(w http.ResponseWriter, r *http.Request) {
	result, err := ctrl.Usecase.GetAsyncServiceResult(r.Context(), id)
	if err != nil {
		utils.BuildErrorResponse(ctrl.Log, w, err)
		return
	}
	utils.BuildSuccessResponse(w, http.StatusOK, "success", map[string]interface{}{
		"service_request_id": result.ServiceRequestID,
		"result":             result.Result,
	})
}
