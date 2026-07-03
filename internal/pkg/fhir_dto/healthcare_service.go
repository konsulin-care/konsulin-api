package fhir_dto

import (
	"errors"
	"strings"
)

// HealthcareService represents a minimal FHIR R4 HealthcareService resource.
// Only fields consumed by the payment flow are deserialised.
type HealthcareService struct {
	ResourceType string      `json:"resourceType,omitempty"`
	ID           string      `json:"id,omitempty"`
	Name         string      `json:"name,omitempty"`
	Extension    []Extension `json:"extension,omitempty"`
}

// ServiceDurationMinutes returns the slot duration in minutes.
// It scans extensions for a URL ending with /fhir/StructureDefinition/serviceDuration.
// Returns error if the extension is missing, value is nil, or value is non-positive.
func (hs *HealthcareService) ServiceDurationMinutes() (int, error) {
	for _, ext := range hs.Extension {
		if strings.HasSuffix(ext.Url, "/fhir/StructureDefinition/serviceDuration") {
			if ext.ValueDuration == nil || ext.ValueDuration.Value == nil {
				return 0, errors.New("serviceDuration extension has no value")
			}
			v := int(*ext.ValueDuration.Value)
			if v <= 0 {
				return 0, errors.New("serviceDuration must be positive")
			}
			return v, nil
		}
	}
	return 0, errors.New("serviceDuration extension not found")
}

// ServiceBufferMinutes returns the buffer duration if present.
// It scans extensions for a URL ending with /fhir/StructureDefinition/serviceBuffer.
// Returns the value and true if found and positive. Returns 0, false if absent or invalid.
func (hs *HealthcareService) ServiceBufferMinutes() (int, bool) {
	for _, ext := range hs.Extension {
		if strings.HasSuffix(ext.Url, "/fhir/StructureDefinition/serviceBuffer") {
			if ext.ValueDuration == nil || ext.ValueDuration.Value == nil {
				return 0, false
			}
			v := int(*ext.ValueDuration.Value)
			if v <= 0 {
				return 0, false
			}
			return v, true
		}
	}
	return 0, false
}
