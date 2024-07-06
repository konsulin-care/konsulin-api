package patients

import (
	"context"
	"encoding/json"
	"konsulin-service/internal/app/services/users"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/dto/responses"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/utils"
)

type patientUsecase struct {
	PatientRepository PatientRepository
	PatientFhirClient PatientFhirClient
	UserRepository    users.UserRepository
}

func NewPatientUsecase(
	patientMongoRepository PatientRepository,
	patientFhirClient PatientFhirClient,
	userRepository users.UserRepository,
) PatientUsecase {
	return &patientUsecase{
		PatientRepository: patientMongoRepository,
		PatientFhirClient: patientFhirClient,
		UserRepository:    userRepository,
	}
}

func (uc *patientUsecase) GetPatientProfileBySession(ctx context.Context, sessionData string) (*responses.PatientProfile, error) {
	var session map[string]interface{}
	err := json.Unmarshal([]byte(sessionData), &session)
	if err != nil {
		return nil, exceptions.WrapWithError(err, constvars.StatusInternalServerError, constvars.ErrClientSomethingWrongWithApplication, constvars.ErrDevServerParseSessionData)
	}

	user := session["user"].(map[string]interface{})
	patientID := user["PatientID"].(string)

	patient, err := uc.PatientFhirClient.GetPatientByID(ctx, patientID)
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

	response := &responses.PatientProfile{
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

func (uc *patientUsecase) UpdatePatientProfileBySession(ctx context.Context, sessionData string, request *requests.UpdateProfile) (*responses.UpdateProfile, error) {
	session, err := utils.ExtractSessionData(sessionData)
	if err != nil {
		return nil, err
	}

	user := session["user"].(map[string]interface{})
	patientID := user["PatientID"].(string)

	// updateData := map[string]interface{}{
	// 	"username": request.Fullname,
	// 	"email":    request.Email,
	// }

	// err = uc.UserRepository.UpdateUser(ctx, user["id"].(string), updateData)
	// if err != nil {
	// 	return nil, err
	// }

	patientFhirRequest := utils.BuildFhirPatientUpdateRequest(request, patientID)

	// Send PUT request to FHIR server to update the patient resource
	fhirPatient, err := uc.PatientFhirClient.UpdatePatient(ctx, patientFhirRequest)
	if err != nil {
		return nil, err
	}

	response := &responses.UpdateProfile{
		PatientID: fhirPatient.ID,
	}

	return response, nil
}
