package utils

import (
	"context"
	"errors"
	"fmt"
	"konsulin-service/internal/pkg/dto/responses"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/fhir_dto"
	"strings"
	"time"
)

func ParseIDFromReference(subject fhir_dto.Reference) (string, error) {
	parts := strings.Split(subject.Reference, "/")
	if len(parts) == 2 {
		return parts[1], nil
	}
	return "", fmt.Errorf("invalid reference format: %s", subject.Reference)
}

func ParseSlashSeparatedToDashSeparated(input string) string {
	parts := strings.Split(input, "/")
	if len(parts) != 2 {
		return input
	}

	processedType := strings.ToLower(
		strings.ReplaceAll(
			strings.ReplaceAll(parts[0], "Item", "-item"),
			"Role", "-role",
		),
	)

	return fmt.Sprintf("%s-%s", processedType, parts[1])
}

func ParseDashSeparatedToSlashSeparated(input string) string {
	lastHyphenIndex := strings.LastIndex(input, "-")
	if lastHyphenIndex == -1 {
		return input
	}

	typePart := input[:lastHyphenIndex]
	idPart := input[lastHyphenIndex+1:]

	typePart = strings.ReplaceAll(typePart, "-item", "Item")
	typePart = strings.ReplaceAll(typePart, "-role", "Role")
	typePart = capitalize(strings.ReplaceAll(typePart, "-", ""))

	return fmt.Sprintf("%s/%s", typePart, idPart)
}

func BuildPatientProfileResponse(patientFhir *fhir_dto.Patient) *responses.UserProfile {
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

func BuildPractitionerProfileResponse(practitionerFhir *fhir_dto.Practitioner) *responses.UserProfile {
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

func ExtractOrganizationIDsFromPractitionerRoles(practitionerRoles []fhir_dto.PractitionerRole) []string {
	organizationIDs := make([]string, 0, len(practitionerRoles))

	for _, role := range practitionerRoles {
		parts := strings.Split(role.Organization.Reference, "/")
		if len(parts) == 2 && parts[0] == "Organization" {
			organizationIDs = append(organizationIDs, parts[1])
		}
	}

	return organizationIDs
}

func ExtractQualifications(qualifications []fhir_dto.Qualification) []string {
	qualificationsResponse := []string{}
	for _, qualification := range qualifications {
		for _, coding := range qualification.Code.Coding {
			qualificationsResponse = append(qualificationsResponse, coding.Display)
		}
	}
	return qualificationsResponse
}

func ExtractSpecialties(specialties []fhir_dto.CodeableConcept) []string {
	qualificationsResponse := []string{}
	for _, specialty := range specialties {
		for _, coding := range specialty.Coding {
			qualificationsResponse = append(qualificationsResponse, coding.Display)
		}
	}
	return qualificationsResponse
}

func ExtractSpecialtiesText(specialties []fhir_dto.CodeableConcept) []string {
	qualificationsResponse := []string{}
	for _, specialty := range specialties {
		qualificationsResponse = append(qualificationsResponse, specialty.Text)
	}
	return qualificationsResponse
}

func MapPractitionerToClinicClinician(practitioner *fhir_dto.Practitioner, specialty []fhir_dto.CodeableConcept, organizationName string) responses.ClinicClinician {
	return responses.ClinicClinician{
		PractitionerID: practitioner.ID,
		Name:           GetFullName(practitioner.Name),
		ClinicName:     organizationName,
		Affiliation:    organizationName,
		Specialties:    ExtractSpecialtiesText(specialty),
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

func GetEducationFromExtensions(extensions []fhir_dto.Extension) []string {
	var educations []string
	for _, ext := range extensions {
		if ext.Url == "http://example.org/fhir/StructureDefinition/education" {
			educations = append(educations, ext.ValueString)
		}
	}
	return educations
}

func GetHomeAddress(addresses []fhir_dto.Address) string {
	for _, address := range addresses {
		if address.Use == "home" {
			return strings.Join(address.Line, ", ")
		}
	}
	return ""
}

func GetWorkAddress(addresses []fhir_dto.Address) string {
	for _, address := range addresses {
		if address.Use == "work" {
			return strings.Join(address.Line, ", ")
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

func GetFullName(names []fhir_dto.HumanName) string {
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

func GetEmailAndWhatsapp(telecoms []fhir_dto.ContactPoint) (string, string) {
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

func DaysContains(slice []string, item string) bool {
	for _, v := range slice {
		switch v {
		case "mon":
			v = time.Monday.String()
		case "tue":
			v = time.Tuesday.String()
		case "wed":
			v = time.Wednesday.String()
		case "thu":
			v = time.Thursday.String()
		case "fri":
			v = time.Friday.String()
		case "sat":
			v = time.Saturday.String()
		case "sun":
			v = time.Sunday.String()
		}
		if v == item {
			return true
		}
	}
	return false
}
func Contains(slice []string, item string) bool {
	for _, v := range slice {
		if v == item {
			return true
		}
	}
	return false
}

func GenerateTimeSlots(start, end string) []string {
	var times []string
	startTime, _ := time.Parse("15:04:05", start)
	endTime, _ := time.Parse("15:04:05", end)

	for t := startTime; t.Before(endTime); t = t.Add(30 * time.Minute) {
		times = append(times, t.Format("15:04"))
	}

	return times
}

func RemoveFromSlice(slice *[]string, item string) {
	for i, v := range *slice {
		if v == item {
			*slice = append((*slice)[:i], (*slice)[i+1:]...)
			break
		}
	}
}

func FindPatientIDFromFhirAppointment(ctx context.Context, request fhir_dto.Appointment) (string, error) {
	for _, participant := range request.Participant {
		if strings.Contains(participant.Actor.Reference, "Patient/") {
			parts := strings.Split(participant.Actor.Reference, "/")
			if len(parts) > 1 {
				return parts[1], nil
			}
		}
	}
	errResponse := errors.New("patient ID not found in appointment")
	return "", exceptions.ErrServerProcess(errResponse)
}

func FindPractitionerIDFromFhirAppointment(ctx context.Context, request fhir_dto.Appointment) (string, error) {
	for _, participant := range request.Participant {
		if strings.Contains(participant.Actor.Reference, "Practitioner/") {
			parts := strings.Split(participant.Actor.Reference, "/")
			if len(parts) > 1 {
				return parts[1], nil
			}
		}
	}
	errResponse := errors.New("practitioner ID not found in appointment")
	return "", exceptions.ErrServerProcess(errResponse)
}

func AddAndGetTime(hoursToAdd, minutesToAdd, secondsToAdd int) string {
	currentTime := time.Now().UTC()

	newTime := currentTime.Add(
		time.Duration(hoursToAdd)*time.Hour +
			time.Duration(minutesToAdd)*time.Minute +
			time.Duration(secondsToAdd)*time.Second)

	return newTime.Format("2006-01-02 15:04:05")
}
