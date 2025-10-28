package fhir_dto

type PractitionerRole struct {
	ResourceType  string            `json:"resourceType,omitempty"`
	ID            string            `json:"id,omitempty"`
	Practitioner  Reference         `json:"practitioner,omitempty"`
	Active        bool              `json:"active,omitempty"`
	Organization  Reference         `json:"organization,omitempty"`
	Specialty     []CodeableConcept `json:"specialty,omitempty"`
	AvailableTime []AvailableTime   `json:"availableTime,omitempty"`
	Extension     []Extension       `json:"extension,omitempty"`
	Period        Period            `json:"period,omitempty"`

// GetPreferredTimezone returns a best-effort timezone derived from the role's period.
func (pr *PractitionerRole) GetPreferredTimezone() (*time.Location, error) {
	if pr == nil {
		return nil, fmt.Errorf("nil practitioner role")
	}
	if pr.Period.Start != "" {
		if t, err := time.Parse(time.RFC3339, pr.Period.Start); err == nil {
			return t.Location(), nil
		}
	}
	if pr.Period.End != "" {
		if t, err := time.Parse(time.RFC3339, pr.Period.End); err == nil {
			return t.Location(), nil
		}
	}
	return nil, fmt.Errorf("cannot determine timezone: invalid period.start and period.end")
}
