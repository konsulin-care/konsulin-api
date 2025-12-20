package requests

type EmailPayload struct {
	Subject  string   `json:"subject"`
	From     string   `json:"from"`
	To       []string `json:"to"`
	Cc       []string `json:"cc,omitempty"`
	Bcc      []string `json:"bcc,omitempty"`
	HTMLCode string   `json:"html_code"`
	Encoded  bool     `json:"encoded"`
}
