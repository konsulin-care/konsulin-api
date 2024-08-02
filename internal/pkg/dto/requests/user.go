package requests

type UpdateProfile struct {
	Fullname           string   `json:"fullname" validate:"required"`
	Email              string   `json:"email" validate:"required,email"`
	BirthDate          string   `json:"birth_date" validate:"required,datetime=2006-01-02"`
	WhatsAppNumber     string   `json:"whatsapp_number" validate:"required"`
	Address            string   `json:"address" validate:"required,max=200"`
	Gender             string   `json:"gender" validate:"required,oneof=male female other unknown"`
	Educations         []string `json:"educations" validate:"required,dive,required"`
	ProfilePicture     []byte   `json:"profile_picture,omitempty"`
	ProfilePictureName string   `json:"profile_picture_name,omitempty"`
	ProfilePictureUrl  string   `json:"profile_picture_url,omitempty"`
}
