package contracts

import (
	"context"
	"konsulin-service/internal/pkg/dto/requests"
)

type WhatsAppSender interface {
	SendWhatsAppMessage(ctx context.Context, request *requests.WhatsAppMessage) error
}
