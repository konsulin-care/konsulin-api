package requests

import "encoding/json"

// NoteStorage is serialized into ServiceRequest.note[0].text
// to persist data needed for later processing.
type NoteStorage struct {
	RawBody        json.RawMessage `json:"rawBody"`
	InstantiateURI string          `json:"instantiateUri"`
	PatientID      string          `json:"patientId"`
	UID            string          `json:"uid"`
}

// CreateServiceRequestStorageInput configures building the ServiceRequest resource
// and its storage payload.
type CreateServiceRequestStorageInput struct {
	UID            string          `json:"uid"`
	PatientID      string          `json:"patientId"`
	InstantiateURI string          `json:"instantiateUri"`
	RawBody        json.RawMessage `json:"rawBody"`
	Occurrence     string          `json:"occurrence"`
}

// CreateServiceRequestStorageOutput returns identifiers and partner transaction ID.
type CreateServiceRequestStorageOutput struct {
	ServiceRequestID      string `json:"serviceRequestId"`
	ServiceRequestVersion string `json:"serviceRequestVersion"`
	PartnerTrxID          string `json:"partnerTrxId"`
}
