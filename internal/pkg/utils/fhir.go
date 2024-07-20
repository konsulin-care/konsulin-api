package utils

import (
	"fmt"
	"konsulin-service/internal/pkg/dto/responses"
	"strings"
	"time"
)

func CalculateAge(birthDate string) int {
	if birthDate == "" {
		return 0
	}

	layout := "2006-01-02"
	dob, err := time.Parse(layout, birthDate)
	if err != nil {
		return 0
	}

	today := time.Now()
	age := today.Year() - dob.Year()
	if today.YearDay() < dob.YearDay() {
		age--
	}

	return age
}

func GetEducationFromExtensions(extensions []responses.Extension) string {
	for _, ext := range extensions {
		if ext.Url == "http://example.org/fhir/StructureDefinition/education" {
			return ext.ValueString
		}
	}
	return ""
}

func GetHomeAddress(addresses []responses.Address) string {
	for _, address := range addresses {
		if address.Use == "home" {
			return fmt.Sprintf("%s, %s, %s, %s, %s",
				strings.Join(address.Line, " "),
				address.City,
				address.State,
				address.PostalCode,
				address.Country,
			)
		}
	}
	return ""
}

func GetWorkAddress(addresses []responses.Address) string {
	for _, address := range addresses {
		if address.Use == "work" {
			return fmt.Sprintf("%s, %s, %s, %s, %s",
				strings.Join(address.Line, " "),
				address.City,
				address.State,
				address.PostalCode,
				address.Country,
			)
		}
	}
	return ""
}

func FormatBirthDate(birthDate string) string {
	if birthDate == "" {
		return ""
	}

	layout := "2006-01-02"
	dob, err := time.Parse(layout, birthDate)
	if err != nil {
		return birthDate
	}

	return dob.Format("02 January 2006")
}

func GetFullName(names []responses.HumanName) string {
	if len(names) == 0 {
		return ""
	}

	var fullname string
	name := names[0]
	if len(name.Given) > 0 {
		fullname += " " + name.Given[0]
	}
	return fullname
}

func GetEmailAndWhatsapp(telecoms []responses.ContactPoint) (string, string) {
	var (
		email          string
		whatsAppNumber string
	)
	for _, telecom := range telecoms {
		switch {
		case telecom.System == "email":
			email = telecom.Value
		case telecom.System == "phone" && telecom.Use == "mobile":
			whatsAppNumber = telecom.Value
		}
	}
	return email, whatsAppNumber
}
