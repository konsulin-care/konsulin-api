package models

import "time"

type Session struct {
	SessionID string
	UserID    string
	ExpiresAt time.Time
}
