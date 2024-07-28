package exceptions

import (
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
	return fmt.Sprintf("%s (%s:%d %s)", e.DevMessage, e.Location.File, e.Location.Line, e.Location.FunctionName)
}

func WrapWithoutError(statusCode int, clientMessage, devMessage string) *CustomError {
	location := getLocation(3)
	return &CustomError{
		StatusCode:    statusCode,
		ClientMessage: clientMessage,
		DevMessage:    devMessage,
		Location:      location,
	}
}

func WrapWithError(err error, statusCode int, clientMessage, devMessage string) *CustomError {
	location := getLocation(3)
	return &CustomError{
		StatusCode:    statusCode,
		ClientMessage: clientMessage,
		DevMessage:    fmt.Sprintf("%s: %s", devMessage, err.Error()),
		Location:      location,
	}
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
