package utils

import (
	"encoding/base64"
	"fmt"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/requests"
)

func BuildForgotPasswordEmailPayload(fromEmail, toEmail, resetLink, userFullName, expiryTime string) *requests.EmailPayload {
	to := []string{toEmail}
	htmlCode := fmt.Sprintf(constvars.EmailSendHTMLSubjectFormat2, userFullName, resetLink, expiryTime)
	encoded := base64.StdEncoding.EncodeToString([]byte(htmlCode))

	return &requests.EmailPayload{
		Subject:  constvars.EmailForgotPasswordSubjectMessage,
		From:     fromEmail,
		To:       to,
		Cc:       []string{},
		Bcc:      []string{},
		HTMLCode: encoded,
		Encoded:  true,
	}
}
