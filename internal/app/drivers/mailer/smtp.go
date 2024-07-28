package mailer

import (
	"konsulin-service/internal/app/config"
	"net/smtp"
)

type SMTPClient struct {
	Host        string
	Port        int
	Username    string
	Password    string
	EmailSender string
	Auth        smtp.Auth
}

func NewSMTPClient(driverConfig *config.DriverConfig) *SMTPClient {
	auth := smtp.PlainAuth("", driverConfig.SMTP.Username, driverConfig.SMTP.Password, driverConfig.SMTP.Host)
	return &SMTPClient{
		Host:        driverConfig.SMTP.Host,
		Port:        driverConfig.SMTP.Port,
		Username:    driverConfig.SMTP.Username,
		Password:    driverConfig.SMTP.Password,
		EmailSender: driverConfig.SMTP.EmailSender,
		Auth:        auth,
	}
}
