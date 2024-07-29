package mailer

import (
	"context"
	"konsulin-service/internal/pkg/dto/requests"
)

type MailerService interface {
	SendEmail(ctx context.Context, request *requests.EmailPayload) error
	ValidateEmail(email string) bool
}
