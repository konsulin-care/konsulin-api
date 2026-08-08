package fhir_dto

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
	return fullName(p.Name)
}

// GetEmailAddresses returns all email values from Telecom where system == email.
func (p Practitioner) GetEmailAddresses() []string {
	return emailsFromTelecom(p.Telecom)
}

// GetPhoneNumbers returns all phone values from Telecom where system == phone.
func (p Practitioner) GetPhoneNumbers() []string {
	return phonesFromTelecom(p.Telecom)
}
