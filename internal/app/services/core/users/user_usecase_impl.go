package users

import (
	"context"
	"encoding/json"
	"konsulin-service/internal/app/models"
	"konsulin-service/internal/app/services/fhir_spark/patients"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/dto/responses"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/utils"
)

type userUsecase struct {
	UserRepository    UserRepository
	PatientFhirClient patients.PatientFhirClient
}

func NewUserUsecase(
	userMongoRepository UserRepository,
	patientFhirClient patients.PatientFhirClient,
) UserUsecase {
	return &userUsecase{
		UserRepository:    userMongoRepository,
		PatientFhirClient: patientFhirClient,
	}
}

func (uc *userUsecase) GetUserProfileBySession(ctx context.Context, sessionData string) (*responses.UserProfile, error) {
	var session models.Session
	err := json.Unmarshal([]byte(sessionData), &session)
	if err != nil {
		return nil, exceptions.ErrCannotParseJSON(err)
	}

	patient, err := uc.PatientFhirClient.GetPatientByID(ctx, session.PatientID)
	if err != nil {
		return nil, err
	}

	fullname := ""
	if len(patient.Name) > 0 {
		fullname = patient.Name[0].Family
		if len(patient.Name[0].Given) > 0 {
			fullname += " " + patient.Name[0].Given[0]
		}
	}

	email := ""
	whatsappNumber := ""
	for _, telecom := range patient.Telecom {
		if telecom.System == "email" {
			email = telecom.Value
		} else if telecom.System == "phone" && telecom.Use == "mobile" {
			whatsappNumber = telecom.Value
		}
	}

	age := utils.CalculateAge(patient.BirthDate)
	education := utils.GetEducationFromExtensions(patient.Extension)
	homeAddress := utils.GetHomeAddress(patient.Address)
	formattedBirthDate := utils.FormatBirthDate(patient.BirthDate)

	response := &responses.UserProfile{
		Fullname:       fullname,
		Email:          email,
		Age:            age,
		Sex:            patient.Gender,
		Education:      education,
		WhatsAppNumber: whatsappNumber,
		HomeAddress:    homeAddress,
		BirthDate:      formattedBirthDate,
	}

	return response, nil
}

func (uc *userUsecase) UpdateUserProfileBySession(ctx context.Context, sessionData string, request *requests.UpdateProfile) (*responses.UpdateUserProfile, error) {
	var session models.Session
	err := json.Unmarshal([]byte(sessionData), &session)
	if err != nil {
		return nil, exceptions.ErrCannotParseJSON(err)
	}

	// Build the update request
	patientFhirRequest := utils.BuildFhirPatientUpdateRequest(request, session.PatientID)

	// Send PUT request to FHIR server to update the user resource
	fhirPatient, err := uc.PatientFhirClient.UpdatePatient(ctx, patientFhirRequest)
	if err != nil {
		return nil, err
	}

	response := &responses.UpdateUserProfile{
		PatientID: fhirPatient.ID,
	}

	return response, nil
}
