package fhir_dto

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
	return fullName(p.Name)
}

// GetEmailAddresses returns all email values from Telecom where system == email.
func (p Person) GetEmailAddresses() []string {
	return emailsFromTelecom(p.Telecom)
}

// GetPhoneNumbers returns all phone values from Telecom where system == phone.
func (p Person) GetPhoneNumbers() []string {
	return phonesFromTelecom(p.Telecom)
}
