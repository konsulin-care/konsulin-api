package exceptions

import (
	"context"
	"fmt"
	"konsulin-service/internal/pkg/constvars"
	"runtime"
)

type CustomError struct {
	StatusCode    int       `json:"status_code"`
	Success       bool      `json:"success"`
	ClientMessage string    `json:"message"`
	DevMessage    string    `json:"dev_message,omitempty"`
	Location      *Location `json:"location,omitempty"`
}

type Location struct {
	File         string `json:"file,omitempty"`
	Line         int    `json:"line,omitempty"`
	FunctionName string `json:"function_name,omitempty"`
}

func (e *CustomError) Error() string {
	return e.DevMessage
}

func BuildNewCustomError(err error, statusCode int, clientMessage, devMessage string) *CustomError {
	location := getLocation(3)
	customError := &CustomError{
		StatusCode:    statusCode,
		ClientMessage: clientMessage,
		DevMessage:    devMessage,
		Location:      location,
	}
	if err != nil {
		if err == context.DeadlineExceeded {
			customError.StatusCode = constvars.StatusGatewayTimeout
			customError.ClientMessage = constvars.ErrClientServerLongRespond
			devMessage = constvars.ErrDevServerDeadlineExceeded
		}
		customError.DevMessage = fmt.Sprintf("%s: %s", devMessage, err.Error())
	}
	return customError
}

func getLocation(skip int) *Location {
	pc, file, line, ok := runtime.Caller(skip)
	if !ok {
		return &Location{
			File:         constvars.ResponseUnknown,
			Line:         0,
			FunctionName: constvars.ResponseUnknown,
		}
	}
	function := runtime.FuncForPC(pc).Name()
	return &Location{
		File:         file,
		Line:         line,
		FunctionName: function,
	}
}
