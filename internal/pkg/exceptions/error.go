package exceptions

import (
	"context"
	"fmt"
	"konsulin-service/internal/pkg/constvars"
	"runtime"
)

type CustomError struct {
	StatusCode    int        `json:"status_code"`
	Success       bool       `json:"success"`
	ClientMessage string     `json:"message"`
	DevMessage    string     `json:"dev_message,omitempty"`
	Locations     []Location `json:"locations,omitempty"`
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
	locations := getLocations(3)
	customError := &CustomError{
		StatusCode:    statusCode,
		ClientMessage: clientMessage,
		DevMessage:    devMessage,
		Locations:     locations,
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

func getLocations(skip int) []Location {
	pc, file, line, ok := runtime.Caller(skip)
	if !ok {
		locations := make([]Location, 0, 1)
		locations = append(locations, Location{
			File:         constvars.ResponseUnknown,
			Line:         0,
			FunctionName: constvars.ResponseUnknown},
		)
		return locations
	}

	locations := make([]Location, 0, 2)
	function := runtime.FuncForPC(pc).Name()
	locations = append(locations, Location{
		File:         file,
		Line:         line,
		FunctionName: function,
	})
	pc, file, line, _ = runtime.Caller(skip - 1)
	function = runtime.FuncForPC(pc).Name()
	locations = append(locations, Location{
		File:         file,
		Line:         line,
		FunctionName: function,
	})

	return locations
}
