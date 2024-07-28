package mailer

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"konsulin-service/internal/app/drivers/mailer"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/exceptions"
	"net/smtp"
	"os"
	"regexp"

	"github.com/rabbitmq/amqp091-go"
)

type mailerService struct {
	Channel *amqp091.Channel
	Client  *mailer.SMTPClient
	Queue   string
}

func NewMailerService(client *mailer.SMTPClient, rabbitMQConnection *amqp091.Connection, queue string) (MailerService, error) {
	channel, err := rabbitMQConnection.Channel()
	if err != nil {
		return nil, err
	}

	return &mailerService{
		Channel: channel,
		Client:  client,
		Queue:   queue,
	}, nil
}
func (s *mailerService) SendEmail(ctx context.Context, request *requests.EmailPayload) error {
	body, err := json.Marshal(request)
	if err != nil {
		return err
	}

	headers := amqp091.Table{
		"message_type":     "JSON",
		"requeue_strategy": "DROP",
	}

	message := amqp091.Publishing{
		ContentType:  constvars.MIMEApplicationJSON,
		Body:         body,
		DeliveryMode: amqp091.Persistent,
		Priority:     0,
		Headers:      headers,
	}

	err = s.Channel.PublishWithContext(ctx, "", s.Queue, false, false, message)
	if err != nil {
		return fmt.Errorf("failed to publish message: %w", err)
	}

	return nil
}

func (svc *mailerService) SendEmail2(to, subject, body string) error {
	from := svc.Client.EmailSender
	msg := []byte(fmt.Sprintf(constvars.EmailSendBasicEmailSubjectFormat, to, subject, body))
	addr := fmt.Sprintf("%s:%d", svc.Client.Host, svc.Client.Port)
	err := smtp.SendMail(addr, svc.Client.Auth, from, []string{to}, msg)
	if err != nil {
		return exceptions.ErrSMTPSendEmail(err, svc.Client.Host)
	}
	return nil
}

func (svc *mailerService) SendHTMLEmail(to, subject, htmlBody string) error {
	from := svc.Client.EmailSender
	msg := []byte(fmt.Sprintf(constvars.EmailSendHTMLSubjectFormat, to, subject, htmlBody))
	addr := fmt.Sprintf("%s:%d", svc.Client.Host, svc.Client.Port)
	err := smtp.SendMail(addr, svc.Client.Auth, from, []string{to}, msg)
	if err != nil {
		return exceptions.ErrSMTPSendEmail(err, svc.Client.Host)
	}
	return nil
}

func (svc *mailerService) SendEmailWithAttachment(to, subject, body, attachmentPath string) error {
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

func (svc *mailerService) ValidateEmail(email string) bool {
	re := regexp.MustCompile(constvars.RegexEmail)
	return re.MatchString(email)
}
