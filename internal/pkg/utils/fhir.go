package utils

import (
	"context"
	"errors"
	"fmt"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/requests"
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

// buildUserProfile assembles a UserProfile response from FHIR demographic
// fields, using addressFormatter to render the address (home vs work).
func buildUserProfile(name []fhir_dto.HumanName, telecom []fhir_dto.ContactPoint, birthDate, gender string, extensions []fhir_dto.Extension, address []fhir_dto.Address, addressFormatter func([]fhir_dto.Address) string) *responses.UserProfile {
	fullname := GetFullName(name)
	email, whatsAppNumber := GetEmailAndWhatsapp(telecom)
	age := CalculateAge(birthDate)
	educations := GetEducationFromExtensions(extensions)
	formattedAddress := addressFormatter(address)
	formattedBirthDate := FormatBirthDate(birthDate)

	return &responses.UserProfile{
		Fullname:       fullname,
		Email:          email,
		Age:            age,
		Gender:         gender,
		Educations:     educations,
		WhatsAppNumber: whatsAppNumber,
		Address:        formattedAddress,
		BirthDate:      formattedBirthDate,
	}
}

func BuildPatientProfileResponse(patientFhir *fhir_dto.Patient) *responses.UserProfile {
	return buildUserProfile(patientFhir.Name, patientFhir.Telecom, patientFhir.BirthDate, patientFhir.Gender, patientFhir.Extension, patientFhir.Address, GetHomeAddress)
}

func BuildPractitionerProfileResponse(practitionerFhir *fhir_dto.Practitioner) *responses.UserProfile {
	return buildUserProfile(practitionerFhir.Name, practitionerFhir.Telecom, practitionerFhir.BirthDate, practitionerFhir.Gender, practitionerFhir.Extension, practitionerFhir.Address, GetWorkAddress)
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
		if ext.Url == constvars.FhirEducationExtensionURL {
			educations = append(educations, ext.ValueString)
		}
	}
	return educations
}

func GetHomeAddress(addresses []fhir_dto.Address) string {
	for _, address := range addresses {
		if address.Use == constvars.FhirAddressUseHome {
			return strings.Join(address.Line, ", ")
		}
	}
	return ""
}

func GetWorkAddress(addresses []fhir_dto.Address) string {
	for _, address := range addresses {
		if address.Use == constvars.FhirAddressUseWork {
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
		case telecom.System == "phone" && telecom.Use == constvars.FhirTelecomUseMobile:
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

// findResourceIDFromAppointment returns the ID portion of the first
// participant actor reference matching resourcePrefix, or a server-process
// error with notFoundMsg when none matches.
func findResourceIDFromAppointment(request fhir_dto.Appointment, resourcePrefix, notFoundMsg string) (string, error) {
	for _, participant := range request.Participant {
		if strings.Contains(participant.Actor.Reference, resourcePrefix) {
			parts := strings.Split(participant.Actor.Reference, "/")
			if len(parts) > 1 {
				return parts[1], nil
			}
		}
	}
	return "", exceptions.ErrServerProcess(errors.New(notFoundMsg))
}

func FindPatientIDFromFhirAppointment(ctx context.Context, request fhir_dto.Appointment) (string, error) {
	return findResourceIDFromAppointment(request, "Patient/", "patient ID not found in appointment")
}

func FindPractitionerIDFromFhirAppointment(ctx context.Context, request fhir_dto.Appointment) (string, error) {
	return findResourceIDFromAppointment(request, "Practitioner/", "practitioner ID not found in appointment")
}

func AddAndGetTime(hoursToAdd, minutesToAdd, secondsToAdd int) string {
	currentTime := time.Now().UTC()

	newTime := currentTime.Add(
		time.Duration(hoursToAdd)*time.Hour +
			time.Duration(minutesToAdd)*time.Minute +
			time.Duration(secondsToAdd)*time.Second)

	return newTime.Format("2006-01-02 15:04:05")
}

func MapJournalRequestToCreateObserVationRequest(request *requests.CreateJournal) (*fhir_dto.Observation, error) {
	journalDate, err := time.Parse("2006-01-02", request.JournalDate)
	if err != nil {
		return nil, err
	}

	components := []fhir_dto.Component{
		{
			Code: fhir_dto.CodeableConcept{
				Text: constvars.FhirObservationJournalTitle,
			},
			ValueString: request.Title,
		},
	}

	for _, body := range request.JournalBody {
		components = append(components, fhir_dto.Component{
			Code: fhir_dto.CodeableConcept{
				Text: constvars.FhirObservationJournalBody,
			},
			ValueString: body,
		})
	}

	observation := &fhir_dto.Observation{
		ResourceType: constvars.ResourceObservation,
		Status:       constvars.FhirObservationStatusFinal,
		Code: fhir_dto.CodeableConcept{
			Coding: []fhir_dto.Coding{
				{
					System:  "https://loinc.org",
					Code:    "51855-5",
					Display: "Patient Note",
				},
			},
			Text: "Patient journaling note",
		},
		Subject: fhir_dto.Reference{
			Reference: fmt.Sprintf("%s/%s", constvars.ResourcePatient, request.PatientID),
		},
		Performer: []fhir_dto.Reference{
			{
				Reference: fmt.Sprintf("%s/%s", constvars.ResourcePatient, request.PatientID),
				Display:   "The patient as performer",
			},
		},
		EffectiveDateTime: journalDate.Format(time.RFC3339),
		Issued:            time.Now().Format(time.RFC3339),
		Component:         components,
	}

	return observation, nil
}

func MapUpdateJournalToUpdateObservationRequest(request *requests.UpdateJournal) (*fhir_dto.Observation, error) {
	journalDate, err := time.Parse("2006-01-02", request.JournalDate)
	if err != nil {
		return nil, exceptions.ErrCannotParseDate(err)
	}

	components := []fhir_dto.Component{
		{
			Code: fhir_dto.CodeableConcept{
				Text: constvars.FhirObservationJournalTitle,
			},
			ValueString: request.Title,
		},
	}

	for _, body := range request.JournalBody {
		components = append(components, fhir_dto.Component{
			Code: fhir_dto.CodeableConcept{
				Text: constvars.FhirObservationJournalBody,
			},
			ValueString: body,
		})
	}

	observation := &fhir_dto.Observation{
		ResourceType: constvars.ResourceObservation,
		ID:           request.JournalID,
		Status:       constvars.FhirObservationStatusAmended,
		Code: fhir_dto.CodeableConcept{
			Coding: []fhir_dto.Coding{
				{
					System:  "https://loinc.org",
					Code:    "51855-5",
					Display: "Patient Note",
				},
			},
			Text: "Patient journaling note",
		},
		Subject: fhir_dto.Reference{
			Reference: fmt.Sprintf("%s/%s", constvars.ResourcePatient, request.PatientID),
		},
		Performer: []fhir_dto.Reference{
			{
				Reference: fmt.Sprintf("%s/%s", constvars.ResourcePatient, request.PatientID),
				Display:   "The patient as performer",
			},
		},
		EffectiveDateTime: journalDate.Format(time.RFC3339),
		Issued:            time.Now().Format(time.RFC3339),
		Component:         components,
	}

	return observation, nil
}

func MapObservationToJournalResponse(observation *fhir_dto.Observation) (*responses.Journal, error) {
	var patientID string
	if observation.Subject.Reference != "" {
		parts := strings.Split(observation.Subject.Reference, "/")
		if len(parts) == 2 {
			patientID = parts[1]
		}
	}

	journalDate, err := time.Parse(time.RFC3339, observation.EffectiveDateTime)
	if err != nil {
		return nil, exceptions.ErrCannotParseDate(err)
	}

	var title string
	var journalBody []string
	for _, component := range observation.Component {
		switch component.Code.Text {
		case constvars.FhirObservationJournalTitle:
			title = component.ValueString
		case constvars.FhirObservationJournalBody:
			journalBody = append(journalBody, component.ValueString)
		}
	}

	journal := &responses.Journal{
		JournalID:   observation.ID,
		PatientID:   patientID,
		Title:       title,
		JournalBody: journalBody,
		JournalDate: journalDate,
	}

	return journal, nil
}

func GetPatientIDFromObservation(observation *fhir_dto.Observation) (string, error) {
	var err error
	if observation.Subject.Reference == "" {
		err = errors.New("subject reference is empty")
		return "", exceptions.ErrServerProcess(err)
	}

	parts := strings.Split(observation.Subject.Reference, "/")
	if len(parts) != 2 || parts[0] != constvars.ResourcePatient {
		err = fmt.Errorf("invalid subject reference format: %s", observation.Subject.Reference)
		return "", exceptions.ErrServerProcess(err)
	}

	return parts[1], nil
}
