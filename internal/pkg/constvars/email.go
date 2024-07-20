package constvars

const (
	EmailForgotPasswordSubjectMessage = "[KONSULIN] Password Reset"
)

const (
	EmailSendWithAttachmentSubjectFormat = "To: %s\r\nSubject: %s\r\nMIME-version: 1.0;\r\nContent-Type: multipart/mixed; boundary=\"simple boundary\"\r\n\r\n--simple boundary\r\nContent-Type: text/plain; charset=\"UTF-8\";\r\n\r\n%s\r\n--simple boundary\r\nContent-Disposition: attachment; filename=\"%s\"\r\nContent-Type: application/octet-stream\r\n\r\n"
	EmailSendHTMLSubjectFormat           = "To: %s\r\nSubject: %s\r\nMIME-version: 1.0;\r\nContent-Type: text/html; charset=\"UTF-8\";\r\n\r\n%s\r\n"
	EmailSendBasicEmailSubjectFormat     = "To: %s\r\nSubject: %s\r\n\r\n%s\r\n"
	EmailBodyResetPassword               = "Click this link to reset your password: %s"
)
