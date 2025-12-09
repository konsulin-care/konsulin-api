package fhir_dto

import "strings"

type Practitioner struct {
	ResourceType string         `json:"resourceType"`
	ID           string         `json:"id,omitempty"`
	Active       bool           `json:"active,omitempty"`
	Name         []HumanName    `json:"name,omitempty"`
	Telecom      []ContactPoint `json:"telecom,omitempty"`
	Gender       string         `json:"gender,omitempty"`
	BirthDate    string         `json:"birthDate,omitempty"`
	Address      []Address      `json:"address,omitempty"`
	Extension    []Extension    `json:"extension,omitempty"`
	Identifier   []Identifier   `json:"identifier"`
}

// FullName returns a best-effort display name for the practitioner.
// Preference: official > usual > first; prefer Text, else Prefix+Given+Family.
func (p Practitioner) FullName() string {
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

// GetEmailAddresses returns all email values from Telecom where system == email.
func (p Practitioner) GetEmailAddresses() []string {
	if len(p.Telecom) == 0 {
		return nil
	}
	emails := make([]string, 0, len(p.Telecom))
	for _, tp := range p.Telecom {
		if tp.System == ContactPointSystemEmail && tp.Value != "" {
			emails = append(emails, tp.Value)
		}
	}
	return emails
}

// GetPhoneNumbers returns all phone values from Telecom where system == phone.
func (p Practitioner) GetPhoneNumbers() []string {
	if len(p.Telecom) == 0 {
		return nil
	}
	phones := make([]string, 0, len(p.Telecom))
	for _, tp := range p.Telecom {
		if tp.System == ContactPointSystemPhone && tp.Value != "" {
			phones = append(phones, tp.Value)
		}
	}
	return phones
}
