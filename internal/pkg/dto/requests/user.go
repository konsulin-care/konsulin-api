package requests

type UpdateProfile struct {
	Fullname                string   `json:"fullname" validate:"required"`
	Email                   string   `json:"email" validate:"required,email"`
	BirthDate               string   `json:"birth_date" validate:"required,datetime=2006-01-02"`
	WhatsAppNumber          string   `json:"whatsapp_number" validate:"required"`
	Address                 string   `json:"address" validate:"required,max=200"`
	Gender                  string   `json:"gender" validate:"required,oneof=male female other unknown"`
	Educations              []string `json:"educations" validate:"required,dive,required"`
	ProfilePicture          string   `json:"profile_picture,omitempty"`
	ProfilePictureExtension string
	ProfilePictureData      []byte
	ProfilePictureMinioUrl  string
}
