package utils

import (
	"errors"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/responses"
	"konsulin-service/internal/pkg/exceptions"
	"net/http"

	"github.com/goccy/go-json"
	"go.uber.org/zap"
)

func BuildSuccessResponse(w http.ResponseWriter, code int, message string, data interface{}) {
	response := responses.ResponseDTO{
		Success: true,
		Message: message,
		Data:    data,
	}
	w.Header().Set(constvars.HeaderContentType, constvars.MIMEApplicationJSON)
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(response)
}

func BuildErrorResponse(log *zap.Logger, w http.ResponseWriter, err error) {
	code := constvars.StatusInternalServerError
	clientMessage := constvars.ErrClientSomethingWrongWithApplication

	var customErr *exceptions.CustomError
	if errors.As(err, &customErr) {
		code = customErr.StatusCode
		clientMessage = customErr.ClientMessage
		location := map[string]interface{}{
			"file":          customErr.Location.File,
			"line":          customErr.Location.Line,
			"function_name": customErr.Location.FunctionName,
		}

		log.Error(customErr.DevMessage,
			zap.Any("location", location),
		)
	} else {
		log.Error(err.Error())
	}

	w.Header().Set(constvars.HeaderContentType, constvars.MIMEApplicationJSON)
	w.WriteHeader(code)
	response := exceptions.CustomError{
		StatusCode:    code,
		Success:       false,
		ClientMessage: clientMessage,
	}

	appEnvironment := GetEnvString("APP_ENV", "development")
	if customErr != nil && appEnvironment != "production" {
		response.DevMessage = customErr.DevMessage
		response.Location = customErr.Location
	}
	json.NewEncoder(w).Encode(response)
}
