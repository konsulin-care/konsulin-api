package utils

import (
	"errors"
	"fmt"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/responses"
	"konsulin-service/internal/pkg/exceptions"
	"net/http"

	"github.com/goccy/go-json"
	"go.uber.org/zap"
)

func BuildPaginationResponse(total, page, pageSize int, baseURL string) *responses.Pagination {
	pagination := &responses.Pagination{
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}

	if (page)*pageSize <= total {
		pagination.NextURL = fmt.Sprintf(constvars.AppPaginationUrlFormat, baseURL, page+1, pageSize)
	}
	if page > 1 {
		pagination.PrevURL = fmt.Sprintf(constvars.AppPaginationUrlFormat, baseURL, page-1, pageSize)
	}

	return pagination
}

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

func BuildSuccessResponseWithPagination(w http.ResponseWriter, code int, message string, pagination *responses.Pagination, data interface{}) {
	response := responses.ResponseDTO{
		Success:    true,
		Message:    message,
		Data:       data,
		Pagination: pagination,
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
		for _, location := range customErr.Locations {
			location := map[string]interface{}{
				"file":          location.File,
				"line":          location.Line,
				"function_name": location.FunctionName,
			}
			log.Error(customErr.DevMessage,
				zap.Any("location", location),
			)
		}

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
		response.Locations = customErr.Locations
	}
	json.NewEncoder(w).Encode(response)
}
