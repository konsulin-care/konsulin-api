package practitioners

import (
	"context"
	"encoding/json"
	"konsulin-service/internal/app/models"
	"konsulin-service/internal/app/services/users"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/dto/responses"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/utils"
)

type practitionerUsecase struct {
	PractitionerRepository PractitionerRepository
	PractitionerFhirClient PractitionerFhirClient
	UserRepository         users.UserRepository
}

func NewPractitionerUsecase(
	practitionerMongoRepository PractitionerRepository,
	practitionerFhirClient PractitionerFhirClient,
	userRepository users.UserRepository,
) PractitionerUsecase {
	return &practitionerUsecase{
		PractitionerRepository: practitionerMongoRepository,
		PractitionerFhirClient: practitionerFhirClient,
		UserRepository:         userRepository,
	}
}

func (uc *practitionerUsecase) GetPractitionerProfileBySession(ctx context.Context, sessionData string) (*responses.PractitionerProfile, error) {
	var session models.Session
	err := json.Unmarshal([]byte(sessionData), &session)
	if err != nil {
		return nil, exceptions.ErrCannotParseJSON(err)
	}

	Practitioner, err := uc.PractitionerFhirClient.GetPractitionerByID(ctx, session.PractitionerID)
	if err != nil {
		return nil, err
	}

	fullname := ""
	if len(Practitioner.Name) > 0 {
		fullname = Practitioner.Name[0].Family
		if len(Practitioner.Name[0].Given) > 0 {
			fullname += " " + Practitioner.Name[0].Given[0]
		}
	}

	email := ""
	whatsappNumber := ""
	for _, telecom := range Practitioner.Telecom {
		if telecom.System == "email" {
			email = telecom.Value
		} else if telecom.System == "phone" && telecom.Use == "mobile" {
			whatsappNumber = telecom.Value
		}
	}

	age := utils.CalculateAge(Practitioner.BirthDate)
	education := utils.GetEducationFromExtensions(Practitioner.Extension)
	homeAddress := utils.GetHomeAddress(Practitioner.Address)
	formattedBirthDate := utils.FormatBirthDate(Practitioner.BirthDate)

	response := &responses.PractitionerProfile{
		Fullname:       fullname,
		Email:          email,
		Age:            age,
		Sex:            Practitioner.Gender,
		Education:      education,
		WhatsAppNumber: whatsappNumber,
		HomeAddress:    homeAddress,
		BirthDate:      formattedBirthDate,
	}

	return response, nil
}

func (uc *practitionerUsecase) UpdatePractitionerProfileBySession(ctx context.Context, sessionData string, request *requests.UpdateProfile) (*responses.UpdatePractitionerProfile, error) {
	var session models.Session
	err := json.Unmarshal([]byte(sessionData), &session)
	if err != nil {
		return nil, exceptions.ErrCannotParseJSON(err)
	}

	// Build the update request
	PractitionerFhirRequest := utils.BuildFhirPractitionerUpdateRequest(request, session.PractitionerID)

	// Send PUT request to FHIR server to update the Practitioner resource
	fhirPractitioner, err := uc.PractitionerFhirClient.UpdatePractitioner(ctx, PractitionerFhirRequest)
	if err != nil {
		return nil, err
	}

	response := &responses.UpdatePractitionerProfile{
		PractitionerID: fhirPractitioner.ID,
	}

	return response, nil
}
