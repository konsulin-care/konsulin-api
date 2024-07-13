package models

type Role struct {
	ID          string       `bson:"_id"`
	Name        string       `bson:"email"`
	Permissions []Permission `bson:"permissions"`
	TimeModel
}

type Permission struct {
	Resource string   `bson:"resource"`
	Actions  []string `bson:"actions"`
}
