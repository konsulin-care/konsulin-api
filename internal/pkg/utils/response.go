package utils

import "konsulin-service/internal/pkg/dto/responses"

func BuildSuccessResponse(message string, data interface{}) responses.ResponseDTO {
	response := responses.ResponseDTO{
		Success: true,
		Message: message,
		Data:    data,
	}
	return response
}
