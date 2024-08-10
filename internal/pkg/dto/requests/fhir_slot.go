package requests

type Slot struct {
	ID       string    `json:"id,omitempty"`
	Schedule Reference `json:"schedule,omitempty"`
	Status   string    `json:"status,omitempty"`
	Start    string    `json:"start,omitempty"`
	End      string    `json:"end,omitempty"`
}
