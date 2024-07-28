package whatsapp

import (
	"context"
	"konsulin-service/internal/pkg/dto/requests"
)

type WhatsAppService interface {
	SendMessage(ctx context.Context, request *requests.WhatsAppMessage) error
}
