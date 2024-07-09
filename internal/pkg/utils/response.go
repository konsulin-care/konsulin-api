package utils

import (
	"errors"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/responses"
	"konsulin-service/internal/pkg/exceptions"
	"net/http"

	"github.com/goccy/go-json"
	"github.com/sirupsen/logrus"
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

func BuildErrorResponse(w http.ResponseWriter, err error) {
	code := constvars.StatusInternalServerError
	clientMessage := constvars.ErrClientSomethingWrongWithApplication

	var customErr *exceptions.CustomError
	if errors.As(err, &customErr) {
		code = customErr.StatusCode
		clientMessage = customErr.ClientMessage
		logrus.WithFields(logrus.Fields{
			"location": logrus.Fields{
				"file":          customErr.Location.File,
				"line":          customErr.Location.Line,
				"function_name": customErr.Location.FunctionName,
			},
		}).Error(customErr.DevMessage)
	} else {
		logrus.Error(err)
	}

	w.Header().Set(constvars.HeaderContentType, constvars.MIMEApplicationJSON)
	w.WriteHeader(code)
	response := exceptions.CustomError{
		StatusCode:    code,
		Success:       false,
		ClientMessage: clientMessage,
	}
	json.NewEncoder(w).Encode(response)
}
