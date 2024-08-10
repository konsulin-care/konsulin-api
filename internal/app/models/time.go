package models

import "time"

type TimeModel struct {
	CreatedAt time.Time  `json:"createdAt" bson:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt" bson:"updatedAt"`
	DeletedAt *time.Time `json:"deletedAt,omitempty" bson:"deletedAt,omitempty"`
}

func (m *TimeModel) SetCreatedAtUpdatedAt() {
	currentTime := time.Now()
	m.CreatedAt = currentTime
	m.UpdatedAt = currentTime
}

func (m *TimeModel) SetUpdatedAt() {
	m.UpdatedAt = time.Now()
}

func (m *TimeModel) SetEmptyDeletedAt() {
	m.DeletedAt = nil
}

func (m *TimeModel) SetDeletedAt() {
	currentTime := time.Now()
	m.DeletedAt = &currentTime
	m.SetUpdatedAt()
}
