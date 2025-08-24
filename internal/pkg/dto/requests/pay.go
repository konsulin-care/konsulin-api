package requests

import "encoding/json"

// CreatePayRequest represents the payload accepted by the /pay endpoint.
type CreatePayRequest struct {
	TotalItem int             `json:"total_item" validate:"required,min=1"`
	Service   string          `json:"service" validate:"required"`
	Body      json.RawMessage `json:"body" validate:"required"`
}
