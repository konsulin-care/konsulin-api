package fhir_dto

import (
	"strings"
)

type Patient struct {
	ID           string         `json:"id,omitempty"`
	ResourceType string         `json:"resourceType,omitempty"`
	Active       bool           `json:"active,omitempty"`
	Name         []HumanName    `json:"name,omitempty"`
	Telecom      []ContactPoint `json:"telecom,omitempty"`
	Gender       string         `json:"gender,omitempty"`
	BirthDate    string         `json:"birthDate,omitempty"`
	Extension    []Extension    `json:"extension,omitempty"`
	Address      []Address      `json:"address,omitempty"`
	Identifier   []Identifier   `json:"identifier"`
}

// FullName returns a best-effort display name for the patient.
// Preference: official > usual > first; prefer Text, else Prefix+Given+Family.
func (p Patient) FullName() string {
	if len(p.Name) == 0 {
		emails := p.GetEmailAddresses()
		for _, email := range emails {
			if strings.Contains(email, "@") {
				firstPart := strings.Split(email, "@")[0]
				if firstPart != "" {
					return firstPart
				}
			}
		}

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
func (p Patient) GetEmailAddresses() []string {
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
func (p Patient) GetPhoneNumbers() []string {
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
