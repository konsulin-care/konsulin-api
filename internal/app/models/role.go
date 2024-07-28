package models

import "konsulin-service/internal/pkg/constvars"

type Role struct {
	ID          string       `bson:"_id"`
	Name        string       `bson:"name"`
	Permissions []Permission `bson:"permissions"`
	TimeModel
}

type Permission struct {
	Resource string   `bson:"resource"`
	Actions  []string `bson:"actions"`
}

func (r *Role) IsNotPatient() bool {
	return r.Name != constvars.RoleTypePatient
}

func (r *Role) IsNotPractitioner() bool {
	return r.Name != constvars.RoleTypePractitioner
}
