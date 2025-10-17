package requests

import "encoding/json"

// NoteStorage is serialized into ServiceRequest.note[0].text
// to persist data needed for later processing.
type NoteStorage struct {
	RawBody json.RawMessage `json:"rawBody"`
	// Deprecated: kept for backward compatibility. New flows store the value in FHIR ServiceRequest.instantiatesUri.
	InstantiateURI string `json:"instantiateUri,omitempty"`
	PatientID      string `json:"patientId"`
	UID            string `json:"uid"`
}

// CreateServiceRequestStorageInput configures building the ServiceRequest resource
// and its storage payload.
type CreateServiceRequestStorageInput struct {
	UID          string `json:"uid"`
	ResourceType string `json:"resourceType"`
	ID           string `json:"id"`
	Subject      string `json:"subject"`
	// instantiatesUri at caller layer is a single string; it will be wrapped to a 0..1 array when sent to FHIR.
	InstantiatesUri string          `json:"instantiatesUri"`
	RawBody         json.RawMessage `json:"rawBody"`
	Occurrence      string          `json:"occurrence"`
}

// CreateServiceRequestStorageOutput returns identifiers and partner transaction ID.
type CreateServiceRequestStorageOutput struct {
	ServiceRequestID      string `json:"serviceRequestId"`
	ServiceRequestVersion string `json:"serviceRequestVersion"`
	PartnerTrxID          string `json:"partnerTrxId"`
	Subject               string `json:"subject"`
}

// GetServiceRequestVersionInput identifies a specific version of a ServiceRequest
type GetServiceRequestVersionInput struct {
	ID      string `json:"id" validate:"required"`
	Version string `json:"version" validate:"required"`
}
