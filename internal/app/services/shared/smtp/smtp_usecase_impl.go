package smtp

import (
	"encoding/base64"
	"fmt"
	"konsulin-service/internal/app/drivers/mailer"
	"konsulin-service/internal/pkg/exceptions"
	"net/smtp"
	"os"
	"regexp"
)

type smtpUsecase struct {
	Client *mailer.SMTPClient
}

func NewSmtpUsecase(client *mailer.SMTPClient) SMTPUsecase {
	return &smtpUsecase{
		Client: client,
	}
}

func (uc *smtpUsecase) SendEmail(to, subject, body string) error {
	from := uc.Client.Username
	msg := []byte(fmt.Sprintf("To: %s\r\nSubject: %s\r\n\r\n%s\r\n", to, subject, body))
	addr := fmt.Sprintf("%s:%d", uc.Client.Host, uc.Client.Port)

	err := smtp.SendMail(addr, uc.Client.Auth, from, []string{to}, msg)
	if err != nil {
		return exceptions.ErrAuthInvalidRole(err)
	}
	return nil
}

func (uc *smtpUsecase) SendHTMLEmail(to, subject, htmlBody string) error {
	from := uc.Client.Username
	msg := []byte(fmt.Sprintf("To: %s\r\nSubject: %s\r\nMIME-version: 1.0;\r\nContent-Type: text/html; charset=\"UTF-8\";\r\n\r\n%s\r\n", to, subject, htmlBody))
	addr := fmt.Sprintf("%s:%d", uc.Client.Host, uc.Client.Port)
	return smtp.SendMail(addr, uc.Client.Auth, from, []string{to}, msg)
}

func (uc *smtpUsecase) SendEmailWithAttachment(to, subject, body, attachmentPath string) error {
	from := uc.Client.Username
	msg := fmt.Sprintf("To: %s\r\nSubject: %s\r\nMIME-version: 1.0;\r\nContent-Type: multipart/mixed; boundary=\"simple boundary\"\r\n\r\n--simple boundary\r\nContent-Type: text/plain; charset=\"UTF-8\";\r\n\r\n%s\r\n--simple boundary\r\nContent-Disposition: attachment; filename=\"%s\"\r\nContent-Type: application/octet-stream\r\n\r\n", to, subject, body, attachmentPath)

	fileContent, err := os.ReadFile(attachmentPath)
	if err != nil {
		return err
	}

	msg += base64.StdEncoding.EncodeToString(fileContent) + "\r\n--simple boundary--"
	return smtp.SendMail(fmt.Sprintf("%s:%d", uc.Client.Host, uc.Client.Port), uc.Client.Auth, from, []string{to}, []byte(msg))
}

func (uc *smtpUsecase) ValidateEmail(email string) bool {
	re := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	return re.MatchString(email)
}
