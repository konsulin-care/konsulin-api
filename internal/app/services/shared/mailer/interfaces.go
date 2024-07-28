package mailer

import (
	"context"
	"konsulin-service/internal/pkg/dto/requests"
)

type MailerService interface {
	SendEmail(ctx context.Context, request *requests.EmailPayload) error
	SendEmail2(to, subject, body string) error
	SendHTMLEmail(to, subject, htmlBody string) error
	SendEmailWithAttachment(to, subject, body, attachmentPath string) error
	ValidateEmail(email string) bool
}
