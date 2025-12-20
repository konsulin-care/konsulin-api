package constvars

const (
	EmailForgotPasswordSubjectMessage           = "[KONSULIN] Password Reset Link"
	EmailPasswordlessSigninupCodeSubjectMessage = "[KONSULIN] Passwordless Code"
	EmailPasswordlessMagicLinkSubjectMessage    = "[KONSULIN] Magic Link Invitation"
)

const (
	EmailSendWithAttachmentSubjectFormat                  = "To: %s\r\nSubject: %s\r\nMIME-version: 1.0;\r\nContent-Type: multipart/mixed; boundary=\"simple boundary\"\r\n\r\n--simple boundary\r\nContent-Type: text/plain; charset=\"UTF-8\";\r\n\r\n%s\r\n--simple boundary\r\nContent-Disposition: attachment; filename=\"%s\"\r\nContent-Type: application/octet-stream\r\n\r\n"
	EmailSendHTMLPasswordlessMagicLinkBodyFormat          = "<html><body>Halo, berikut adalah link untuk bergabung ke dalam aplikasi Konsulin:<br><br>%s<br><br> Terima kasih telah memilih Konsulin.</body></html>"
	EmailSendHTMLForgotPasswordBodyFormat                 = "<html><body>Halo, berikut adalah link untuk melakukan reset ulang kada sandi Anda:<br><br>%s<br><br>Kode ini valid hingga %s dan hanya bisa digunakan sekali. Jika anda tidak merasa melakukan aksi ini mohon abaikan email ini.</body></html>"
	EmailSendHTMLForgotPasswordBodyFormatWithUserFullname = "<html> <body> Halo <strong>%s</strong>. Berikut adalah link untuk melakukan reset ulang kada sandi Anda: <br> <br> %s <br> <br> Kode ini valid hingga %s dan hanya bisa digunakan sekali. Jika anda tidak merasa melakukan aksi ini mohon abaikan email ini. </body> </html>"
	EmailSendBasicEmailSubjectFormat                      = "To: %s\r\nSubject: %s\r\n\r\n%s\r\n"
	EmailBodyResetPassword                                = "Click this link to reset your password: %s"
)
