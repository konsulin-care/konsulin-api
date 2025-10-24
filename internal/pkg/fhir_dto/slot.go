package fhir_dto

import "time"

// SlotStatus enumerates valid FHIR Slot.status values.
// docs: https://hl7.org/fhir/R4/valueset-slotstatus.html
type SlotStatus string

const (
	SlotStatusBusy            SlotStatus = "busy"
	SlotStatusFree            SlotStatus = "free"
	SlotStatusBusyUnavailable SlotStatus = "busy-unavailable"
	SlotStatusBusyTentative   SlotStatus = "busy-tentative"
	SlotStatusEnteredInError  SlotStatus = "entered-in-error"
)

type Slot struct {
	ResourceType string       `json:"resourceType" bson:"resourceType"`
	ID           string       `json:"id,omitempty" bson:"id,omitempty"`
	Meta         Meta         `json:"meta,omitempty" bson:"meta,omitempty"`
	Identifier   []Identifier `json:"identifier,omitempty" bson:"identifier,omitempty"`
	Schedule     Reference    `json:"schedule" bson:"schedule"`
	Status       SlotStatus   `json:"status" bson:"status"`
	Start        time.Time    `json:"start" bson:"start"`
	End          time.Time    `json:"end" bson:"end"`
	Overbooked   bool         `json:"overbooked,omitempty" bson:"overbooked,omitempty"`
	Comment      string       `json:"comment,omitempty" bson:"comment,omitempty"`
}
