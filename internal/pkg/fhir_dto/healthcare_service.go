package fhir_dto

import (
	"errors"
	"konsulin-service/internal/pkg/constvars"
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

// isDurationMinutes checks whether the Duration unit/code represents minutes.
// Returns true if unit/code is empty (backwards compatible) or explicitly minutes.
func isDurationMinutes(d *Duration) bool {
	if d.Unit != "" && d.Unit != constvars.HealthcareServiceUnitMinutes {
		return false
	}
	if d.Code != "" && d.Code != constvars.HealthcareServiceCodeMin {
		return false
	}
	return true
}

// findDurationExtension scans extensions for one ending with the given suffix.
// Returns the positive integer value and true if the extension exists, has a
// ValueDuration in minutes, and has a positive value.
func (hs *HealthcareService) findDurationExtension(suffix string) (int, bool) {
	for _, ext := range hs.Extension {
		if strings.HasSuffix(ext.Url, suffix) {
			if ext.ValueDuration == nil || ext.ValueDuration.Value == nil {
				return 0, false
			}
			if !isDurationMinutes(ext.ValueDuration) {
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

// ServiceDurationMinutes returns the slot duration in minutes.
// It scans extensions for a URL ending with /fhir/StructureDefinition/serviceDuration.
// Returns error if the extension is missing, value is nil, non-positive, or not in minutes.
func (hs *HealthcareService) ServiceDurationMinutes() (int, error) {
	v, ok := hs.findDurationExtension("/fhir/StructureDefinition/serviceDuration")
	if !ok {
		return 0, errors.New("serviceDuration extension not found or has no valid value in minutes")
	}
	return v, nil
}

// ServiceBufferMinutes returns the buffer duration if present.
// It scans extensions for a URL ending with /fhir/StructureDefinition/serviceBuffer.
// Returns the value and true if found, positive, and in minutes. Returns 0, false if absent or invalid.
func (hs *HealthcareService) ServiceBufferMinutes() (int, bool) {
	return hs.findDurationExtension("/fhir/StructureDefinition/serviceBuffer")
}
