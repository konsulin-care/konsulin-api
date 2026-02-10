package exceptions

import (
	"context"
	"errors"
	"fmt"
	"konsulin-service/internal/pkg/constvars"
	"runtime"
	"strings"
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

// IsHTTPErrRetryable reports whether err is a retriable HTTP transport error.
//
// It is intended only for outbound HTTP operations (e.g. client.Do, sending a request
// to a remote server). It returns true when err was produced by a failure to send the
// HTTP request (e.g. connection refused, timeout, temporary network failure). Callers
// may use this to decide whether to retry the same HTTP operation.
//
// This function should still retain the functionality even when the
// exceptions package is refactored to use a different error type.
//
// It returns false for all other errors, including:
//   - Database, cache, or other non-HTTP I/O errors
//   - HTTP response errors (4xx/5xx from the server)
//   - Request build or marshal errors
//   - Response decode errors
//
// Do not use this for database ops, file/memory reads, or any non-HTTP flow. Use it
// only where an HTTP client send failed and a retry of the same request is safe and
// meaningful.
func IsHTTPErrRetryable(err error) bool {
	if err == nil {
		return false
	}
	var ce *CustomError
	if !errors.As(err, &ce) {
		return false
	}
	return strings.Contains(ce.DevMessage, constvars.ErrDevSendHTTPRequest)
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
	// pc, file, line, _ = runtime.Caller(skip - 1)
	// function = runtime.FuncForPC(pc).Name()
	// locations = append(locations, Location{
	// 	File:         file,
	// 	Line:         line,
	// 	FunctionName: function,
	// })

	return locations
}
