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

type smtpUsecase struct {
	Client *mailer.SMTPClient
}

func NewSmtpUsecase(client *mailer.SMTPClient) SMTPUsecase {
	return &smtpUsecase{
		Client: client,
	}
}

func (uc *smtpUsecase) SendEmail(to, subject, body string) error {
	from := uc.Client.EmailSender
	msg := []byte(fmt.Sprintf(constvars.EmailSendBasicEmailSubjectFormat, to, subject, body))
	addr := fmt.Sprintf("%s:%d", uc.Client.Host, uc.Client.Port)

	err := smtp.SendMail(addr, uc.Client.Auth, from, []string{to}, msg)
	if err != nil {
		return exceptions.ErrSMTPSendEmail(err, uc.Client.Host)
	}
	return nil
}

func (uc *smtpUsecase) SendHTMLEmail(to, subject, htmlBody string) error {
	from := uc.Client.EmailSender
	msg := []byte(fmt.Sprintf(constvars.EmailSendHTMLSubjectFormat, to, subject, htmlBody))
	addr := fmt.Sprintf("%s:%d", uc.Client.Host, uc.Client.Port)
	err := smtp.SendMail(addr, uc.Client.Auth, from, []string{to}, msg)
	if err != nil {
		return exceptions.ErrSMTPSendEmail(err, uc.Client.Host)
	}
	return nil
}

func (uc *smtpUsecase) SendEmailWithAttachment(to, subject, body, attachmentPath string) error {
	from := uc.Client.EmailSender
	msg := fmt.Sprintf(constvars.EmailSendWithAttachmentSubjectFormat, to, subject, body, attachmentPath)

	fileContent, err := os.ReadFile(attachmentPath)
	if err != nil {
		return exceptions.ErrServerProcess(err)
	}
	msg += base64.StdEncoding.EncodeToString(fileContent) + "\r\n--simple boundary--"
	err = smtp.SendMail(fmt.Sprintf("%s:%d", uc.Client.Host, uc.Client.Port), uc.Client.Auth, from, []string{to}, []byte(msg))
	if err != nil {
		return exceptions.ErrSMTPSendEmail(err, uc.Client.Host)
	}
	return nil

}

func (uc *smtpUsecase) ValidateEmail(email string) bool {
	re := regexp.MustCompile(constvars.RegexEmail)
	return re.MatchString(email)
}
