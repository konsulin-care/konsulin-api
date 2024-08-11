package responses

import "time"

type Appointment struct {
	ResourceType       string                   `json:"resourceType"`
	ID                 string                   `json:"id,omitempty"`
	Meta               Meta                     `json:"meta,omitempty"`
	Status             string                   `json:"status"`
	ServiceCategory    []CodeableConcept        `json:"serviceCategory,omitempty"`
	ServiceType        []CodeableConcept        `json:"serviceType,omitempty"`
	Specialty          []CodeableConcept        `json:"specialty,omitempty"`
	AppointmentType    CodeableConcept          `json:"appointmentType,omitempty"`
	ReasonCode         []CodeableConcept        `json:"reasonCode,omitempty"`
	ReasonReference    []Reference              `json:"reasonReference,omitempty"`
	Priority           uint                     `json:"priority,omitempty"`
	Description        string                   `json:"description,omitempty"`
	Start              time.Time                `json:"start,omitempty"`
	End                time.Time                `json:"end,omitempty"`
	MinutesDuration    uint                     `json:"minutesDuration,omitempty"`
	Slot               []Reference              `json:"slot,omitempty"`
	Created            time.Time                `json:"created,omitempty"`
	Comment            string                   `json:"comment,omitempty"`
	PatientInstruction string                   `json:"patientInstruction,omitempty"`
	BasedOn            []Reference              `json:"basedOn,omitempty"`
	Participant        []AppointmentParticipant `json:"participant,omitempty"`
	RequestedPeriod    []Period                 `json:"requestedPeriod,omitempty"`
}

type AppointmentParticipant struct {
	Type     []CodeableConcept `json:"type,omitempty"`
	Actor    Reference         `json:"actor,omitempty"`
	Required string            `json:"required,omitempty"`
	Status   string            `json:"status"`
	Period   Period            `json:"period,omitempty"`
}
