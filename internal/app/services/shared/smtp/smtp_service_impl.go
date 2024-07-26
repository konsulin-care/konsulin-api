package smtp

import (
	"encoding/base64"
	"fmt"
	"konsulin-service/internal/app/drivers/mailer"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/exceptions"
	"net/smtp"
	"os"
	"regexp"
)

type smtpService struct {
	Client *mailer.SMTPClient
}

func NewSmtpService(client *mailer.SMTPClient) SMTPService {
	return &smtpService{
		Client: client,
	}
}

func (svc *smtpService) SendEmail(to, subject, body string) error {
	from := svc.Client.EmailSender
	msg := []byte(fmt.Sprintf(constvars.EmailSendBasicEmailSubjectFormat, to, subject, body))
	addr := fmt.Sprintf("%s:%d", svc.Client.Host, svc.Client.Port)
	err := smtp.SendMail(addr, svc.Client.Auth, from, []string{to}, msg)
	if err != nil {
		return exceptions.ErrSMTPSendEmail(err, svc.Client.Host)
	}
	return nil
}

func (svc *smtpService) SendHTMLEmail(to, subject, htmlBody string) error {
	from := svc.Client.EmailSender
	msg := []byte(fmt.Sprintf(constvars.EmailSendHTMLSubjectFormat, to, subject, htmlBody))
	addr := fmt.Sprintf("%s:%d", svc.Client.Host, svc.Client.Port)
	err := smtp.SendMail(addr, svc.Client.Auth, from, []string{to}, msg)
	if err != nil {
		return exceptions.ErrSMTPSendEmail(err, svc.Client.Host)
	}
	return nil
}

func (svc *smtpService) SendEmailWithAttachment(to, subject, body, attachmentPath string) error {
	from := svc.Client.EmailSender
	msg := fmt.Sprintf(constvars.EmailSendWithAttachmentSubjectFormat, to, subject, body, attachmentPath)

	fileContent, err := os.ReadFile(attachmentPath)
	if err != nil {
		return exceptions.ErrServerProcess(err)
	}
	msg += base64.StdEncoding.EncodeToString(fileContent) + "\r\n--simple boundary--"
	err = smtp.SendMail(fmt.Sprintf("%s:%d", svc.Client.Host, svc.Client.Port), svc.Client.Auth, from, []string{to}, []byte(msg))
	if err != nil {
		return exceptions.ErrSMTPSendEmail(err, svc.Client.Host)
	}
	return nil

}

func (svc *smtpService) ValidateEmail(email string) bool {
	re := regexp.MustCompile(constvars.RegexEmail)
	return re.MatchString(email)
}
