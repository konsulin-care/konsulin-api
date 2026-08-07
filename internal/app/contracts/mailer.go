package contracts

import (
	"context"
	"konsulin-service/internal/pkg/dto/requests"
)

type EmailSender interface {
	SendEmail(ctx context.Context, request *requests.EmailPayload) error
}
