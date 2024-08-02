package responses

import (
	"strings"
)

type Organization struct {
	ResourceType string            `json:"resourceType,omitempty"`
	ID           string            `json:"id,omitempty"`
	Active       bool              `json:"active,omitempty"`
	Identifier   []Identifier      `json:"identifier,omitempty"`
	Type         []CodeableConcept `json:"type,omitempty"`
	Name         string            `json:"name,omitempty"`
	Alias        []string          `json:"alias,omitempty"`
	Telecom      []ContactPoint    `json:"telecom,omitempty"`
	Address      []Address         `json:"address,omitempty"`
	PartOf       Reference         `json:"partOf,omitempty"`
}

func (org *Organization) ConvertToClinicResponse() Clinic {
	clinic := Clinic{
		ID:          org.ID,
		ClinicName:  org.Name,
		Affiliation: org.PartOf.Display,
		Tags:        org.Alias,
	}

	if len(org.Address) > 0 {
		clinic.Address = strings.Join(org.Address[0].Line, ", ")
		if org.Address[0].City != "" {
			clinic.Address += ", " + org.Address[0].City
		}
		if org.Address[0].State != "" {
			clinic.Address += ", " + org.Address[0].State
		}
		if org.Address[0].PostalCode != "" {
			clinic.Address += " " + org.Address[0].PostalCode
		}
		if org.Address[0].Country != "" {
			clinic.Address += ", " + org.Address[0].Country
		}
	}

	return clinic
}
