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
// Falls back to the email local part when no name is present.
func (p Patient) FullName() string {
	if len(p.Name) == 0 {
		return p.emailFallbackFullName()
	}
	return fullName(p.Name)
}

// preferredName selects the best HumanName from a slice.
// Priority: official > usual > first entry.
func preferredName(names []HumanName) HumanName {
	for _, n := range names {
		if strings.EqualFold(n.Use, "official") {
			return n
		}
	}
	for _, n := range names {
		if strings.EqualFold(n.Use, "usual") {
			return n
		}
	}
	return names[0]
}

// formatHumanName builds a display string from a HumanName's Prefix+Given+Family.
func formatHumanName(n HumanName) string {
	parts := make([]string, 0, 3)
	if len(n.Prefix) > 0 {
		parts = append(parts, strings.Join(n.Prefix, " "))
	}
	if len(n.Given) > 0 {
		parts = append(parts, strings.Join(n.Given, " "))
	}
	if s := strings.TrimSpace(n.Family); s != "" {
		parts = append(parts, s)
	}
	return strings.TrimSpace(strings.Join(parts, " "))
}

// emailFallbackFullName returns the local part of the first email address.
func (p Patient) emailFallbackFullName() string {
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

// GetEmailAddresses returns all email values from Telecom where system == email.
func (p Patient) GetEmailAddresses() []string {
	return emailsFromTelecom(p.Telecom)
}

// GetPhoneNumbers returns all phone values from Telecom where system == phone.
func (p Patient) GetPhoneNumbers() []string {
	return phonesFromTelecom(p.Telecom)
}
