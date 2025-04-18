package responses

type SupertokenCreateCode struct {
	CodeID           string `json:"code_id"`
	DeviceID         string `json:"device_id"`
	PreAuthSessionID string `json:"pre_auth_session_id"`
}

type SupertokenPlessUser struct {
	ID          string   `json:"id"`
	Email       string   `json:"email"`
	PhoneNumber string   `json:"phone_number"`
	TimeJoined  uint64   `json:"time_joined"`
	TenantIds   []string `json:"tenant_ids"`
}

type SupertokenConsumeCode struct {
	User           SupertokenPlessUser `json:"user"`
	CreatedNewUser bool                `json:"created_new_user"`
}
