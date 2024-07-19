package smtp

type SMTPUsecase interface {
	SendEmail(to, subject, body string) error
	SendHTMLEmail(to, subject, htmlBody string) error
	SendEmailWithAttachment(to, subject, body, attachmentPath string) error
	ValidateEmail(email string) bool
}
