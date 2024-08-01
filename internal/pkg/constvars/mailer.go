package constvars

const (
	EmailForgotPasswordSubjectMessage = "[KONSULIN] Password Reset Link"
)

const (
	EmailSendWithAttachmentSubjectFormat = "To: %s\r\nSubject: %s\r\nMIME-version: 1.0;\r\nContent-Type: multipart/mixed; boundary=\"simple boundary\"\r\n\r\n--simple boundary\r\nContent-Type: text/plain; charset=\"UTF-8\";\r\n\r\n%s\r\n--simple boundary\r\nContent-Disposition: attachment; filename=\"%s\"\r\nContent-Type: application/octet-stream\r\n\r\n"
	EmailSendHTMLSubjectFormat           = "<html><body>Salam, <strong>%s</strong>, Berikut adalah link untuk melakukan reset ulang kada sandi Anda:<br><br>%s<br><br>Kode ini valid hingga %s dan hanya bisa digunakan sekali. Jika anda tidak merasa melakukan aksi ini mohon abaikan email ini.</body></html>"
	EmailSendHTMLSubjectFormat2          = "<html> <body> Salam <strong>%s</strong>, Berikut adalah link untuk melakukan reset ulang kada sandi Anda: <br> <br> %s <br> <br> Kode ini valid hingga %s dan hanya bisa digunakan sekali. Jika anda tidak merasa melakukan aksi ini mohon abaikan email ini. </body> </html>"
	EmailSendBasicEmailSubjectFormat     = "To: %s\r\nSubject: %s\r\n\r\n%s\r\n"
	EmailBodyResetPassword               = "Click this link to reset your password: %s"
)
