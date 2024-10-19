package whatsapp

import (
	"context"
	"konsulin-service/internal/pkg/dto/requests"
)

type WhatsAppService interface {
	SendWhatsAppMessage(ctx context.Context, request *requests.WhatsAppMessage) error
}
