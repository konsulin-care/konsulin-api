package utils

import (
	"fmt"
	"konsulin-service/internal/pkg/dto/responses"
	"strings"
	"time"
)

func BuildPatientProfileResponse(patientFhir *responses.Patient) *responses.UserProfile {
	fullname := GetFullName(patientFhir.Name)
	email, whatsAppNumber := GetEmailAndWhatsapp(patientFhir.Telecom)
	age := CalculateAge(patientFhir.BirthDate)
	educations := GetEducationFromExtensions(patientFhir.Extension)
	formattedAddress := GetHomeAddress(patientFhir.Address)
	formattedBirthDate := FormatBirthDate(patientFhir.BirthDate)

	return &responses.UserProfile{
		Fullname:       fullname,
		Email:          email,
		Age:            age,
		Gender:         patientFhir.Gender,
		Educations:     educations,
		WhatsAppNumber: whatsAppNumber,
		Address:        formattedAddress,
		BirthDate:      formattedBirthDate,
	}
}

func BuildPractitionerProfileResponse(practitionerFhir *responses.Practitioner) *responses.UserProfile {
	fullname := GetFullName(practitionerFhir.Name)
	email, whatsAppNumber := GetEmailAndWhatsapp(practitionerFhir.Telecom)
	age := CalculateAge(practitionerFhir.BirthDate)
	educations := GetEducationFromExtensions(practitionerFhir.Extension)
	formattedAddress := GetWorkAddress(practitionerFhir.Address)
	formattedBirthDate := FormatBirthDate(practitionerFhir.BirthDate)

	return &responses.UserProfile{
		Fullname:       fullname,
		Email:          email,
		Age:            age,
		Gender:         practitionerFhir.Gender,
		Educations:     educations,
		WhatsAppNumber: whatsAppNumber,
		Address:        formattedAddress,
		BirthDate:      formattedBirthDate,
	}
}

func ExtractOrganizationIDsFromPractitionerRoles(practitionerRoles []responses.PractitionerRole) []string {
	organizationIDs := make([]string, 0, len(practitionerRoles))

	for _, role := range practitionerRoles {
		parts := strings.Split(role.Organization.Reference, "/")
		if len(parts) == 2 && parts[0] == "Organization" {
			organizationIDs = append(organizationIDs, parts[1])
		}
	}

	return organizationIDs
}

func ExtractQualifications(qualifications []responses.Qualification) []string {
	qualificationsResponse := []string{}
	for _, qualification := range qualifications {
		for _, coding := range qualification.Code.Coding {
			qualificationsResponse = append(qualificationsResponse, coding.Display)
		}
	}
	return qualificationsResponse
}

func ExtractSpecialties(specialties []responses.CodeableConcept) []string {
	qualificationsResponse := []string{}
	for _, specialty := range specialties {
		for _, coding := range specialty.Coding {
			qualificationsResponse = append(qualificationsResponse, coding.Display)
		}
	}
	return qualificationsResponse
}

func MapPractitionerToClinicClinician(practitioner *responses.Practitioner, specialty []responses.CodeableConcept, organizationName string) responses.ClinicClinician {
	return responses.ClinicClinician{
		PractitionerID: practitioner.ID,
		Name:           GetFullName(practitioner.Name),
		ClinicName:     organizationName,
		Affiliation:    organizationName,
		Specialties:    ExtractSpecialties(specialty),
	}
}

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

func GetEducationFromExtensions(extensions []responses.Extension) []string {
	var educations []string
	for _, ext := range extensions {
		if ext.Url == "http://example.org/fhir/StructureDefinition/education" {
			educations = append(educations, ext.ValueString)
		}
	}
	return educations
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

	if len(name.Prefix) > 0 {
		fullname += name.Prefix[0] + " "
	}
	if len(name.Given) > 0 {
		fullname += name.Given[0]
	}

	if name.Family != "" {
		fullname += " " + name.Family
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

func Contains(slice []string, item string) bool {
	for _, v := range slice {
		if v == item {
			return true
		}
	}
	return false
}
