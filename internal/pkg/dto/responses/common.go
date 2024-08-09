package responses

import "encoding/json"

type FHIRBundle struct {
	ResourceType string  `json:"resourceType"`
	ID           string  `json:"id"`
	Type         string  `json:"type"`
	Total        int     `json:"total"`
	Entry        []Entry `json:"entry"`
}

type Entry struct {
	Resource json.RawMessage `json:"resource"`
}
type ResponseDTO struct {
	Success    bool        `json:"success"`
	Message    string      `json:"message,omitempty"`
	Data       interface{} `json:"data,omitempty"`
	Pagination *Pagination `json:"pagination,omitempty"`
}

type Pagination struct {
	Total    int    `json:"total"`
	Page     int    `json:"page"`
	PageSize int    `json:"page_size"`
	NextURL  string `json:"next_url,omitempty"`
	PrevURL  string `json:"prev_url,omitempty"`
}
