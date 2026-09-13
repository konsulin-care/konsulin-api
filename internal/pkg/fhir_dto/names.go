package fhir_dto

import "strings"

// fullName builds a best-effort display name from a slice of HumanName.
// Preference: official > usual > first; prefer Text, else Prefix+Given+Family.
// Returns "" when the slice is empty.
func fullName(names []HumanName) string {
	if len(names) == 0 {
		return ""
	}
	chosen := preferredName(names)
	if s := strings.TrimSpace(chosen.Text); s != "" {
		return s
	}
	return formatHumanName(chosen)
}

// valuesFromTelecom returns all Telecom values whose system matches, skipping
// empty values.
func valuesFromTelecom(telecom []ContactPoint, system ContactPointSystemCode) []string {
	if len(telecom) == 0 {
		return nil
	}
	values := make([]string, 0, len(telecom))
	for _, tp := range telecom {
		if tp.System == system && tp.Value != "" {
			values = append(values, tp.Value)
		}
	}
	return values
}

// emailsFromTelecom returns all email values from Telecom where system == email.
func emailsFromTelecom(telecom []ContactPoint) []string {
	return valuesFromTelecom(telecom, ContactPointSystemEmail)
}

// phonesFromTelecom returns all phone values from Telecom where system == phone.
func phonesFromTelecom(telecom []ContactPoint) []string {
	return valuesFromTelecom(telecom, ContactPointSystemPhone)
}
