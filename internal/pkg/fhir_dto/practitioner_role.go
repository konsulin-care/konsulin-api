package fhir_dto

import (
	"fmt"
	"time"
)

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
	NotAvailable  []NotAvailable    `json:"notAvailable,omitempty"`
}

type NotAvailable struct {
	Description string `json:"description"`
	During      Period `json:"during,omitempty"`
}

// AddNotAvailable appends or merges a notAvailable period with the same description.
func (pr *PractitionerRole) AddNotAvailable(description string, start, end time.Time) {
	if pr == nil {
		return
	}
	// Normalize to RFC3339
	ns := start.Format(time.RFC3339)
	ne := end.Format(time.RFC3339)
	pr.NotAvailable = append(pr.NotAvailable, NotAvailable{
		Description: description,
		During: Period{
			Start: ns,
			End:   ne,
		},
	})
}

// RemoveOutdatedNotAvailableReasons removes entries fully in the past (end < now).
func (pr *PractitionerRole) RemoveOutdatedNotAvailableReasons() error {
	if pr == nil || len(pr.NotAvailable) == 0 {
		return nil
	}
	loc, err := pr.GetPreferredTimezone()
	if err != nil {
		return err
	}
	now := time.Now().In(loc)
	filtered := make([]NotAvailable, 0, len(pr.NotAvailable))
	for _, na := range pr.NotAvailable {
		if na.During.End == "" {
			filtered = append(filtered, na)
			continue
		}
		t, perr := time.Parse(time.RFC3339, na.During.End)
		if perr != nil {
			// keep malformed to avoid unintended data loss
			filtered = append(filtered, na)
			continue
		}
		if t.After(now) || t.Equal(now) {
			filtered = append(filtered, na)
		}
	}
	pr.NotAvailable = filtered
	return nil
}

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
