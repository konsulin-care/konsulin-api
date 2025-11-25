package fhir_dto

import "strings"

type Person struct {
	ResourceType         string         `json:"resourceType"`
	ID                   string         `json:"id,omitempty"`
	Active               bool           `json:"active,omitempty"`
	Name                 []HumanName    `json:"name,omitempty"`
	Telecom              []ContactPoint `json:"telecom,omitempty"`
	Gender               string         `json:"gender,omitempty"`
	BirthDate            string         `json:"birthDate,omitempty"`
	Photo                *Attachment    `json:"photo,omitempty"`
	Identifier           []Identifier   `json:"identifier"`
	ManagingOrganization *Reference     `json:"managingOrganization,omitempty"`
}

// FullName returns a best-effort display name for the person.
// Preference: official > usual > first; prefer Text, else Prefix+Given+Family.
func (p Person) FullName() string {
	if len(p.Name) == 0 {
		return ""
	}
	chosen := p.Name[0]
	for _, n := range p.Name {
		if strings.EqualFold(n.Use, "official") {
			chosen = n
			break
		}
	}
	if !strings.EqualFold(chosen.Use, "official") {
		for _, n := range p.Name {
			if strings.EqualFold(n.Use, "usual") {
				chosen = n
				break
			}
		}
	}
	if s := strings.TrimSpace(chosen.Text); s != "" {
		return s
	}
	parts := []string{}
	if len(chosen.Prefix) > 0 {
		parts = append(parts, strings.Join(chosen.Prefix, " "))
	}
	if len(chosen.Given) > 0 {
		parts = append(parts, strings.Join(chosen.Given, " "))
	}
	if s := strings.TrimSpace(chosen.Family); s != "" {
		parts = append(parts, s)
	}
	return strings.TrimSpace(strings.Join(parts, " "))
}
