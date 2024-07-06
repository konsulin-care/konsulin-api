package utils

import (
	"encoding/json"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/exceptions"
	"strings"
)

func BuildFhirPatientRequest(username, email string) *requests.PatientFhir {
	return &requests.PatientFhir{
		ResourceType: constvars.ResourcePatient,
		Telecom: []requests.ContactPoint{
			{
				System: "email",
				Value:  email,
				Use:    "home",
			},
		},
	}
}

func ExtractSessionData(sessionData string) (map[string]interface{}, error) {
	var session map[string]interface{}
	err := json.Unmarshal([]byte(sessionData), &session)
	if err != nil {
		return nil, exceptions.WrapWithError(err, constvars.StatusInternalServerError, constvars.ErrClientSomethingWrongWithApplication, constvars.ErrDevServerParseSessionData)
	}
	return session, err
}

func BuildFhirPatientUpdateRequest(request *requests.UpdateProfile, patientID string) *requests.PatientFhir {
	return &requests.PatientFhir{
		ResourceType: constvars.ResourcePatient,
		ID:           patientID,
		Active:       true,
		Name: []requests.HumanName{
			{
				Use:    "official",
				Family: request.Fullname,
				Given:  []string{request.Fullname},
			},
		},
		Telecom: []requests.ContactPoint{
			{
				System: "email",
				Value:  request.Email,
				Use:    "home",
			},
			{
				System: "phone",
				Value:  request.WhatsAppNumber,
				Use:    "mobile",
			},
		},
		Gender:    request.Gender,
		BirthDate: request.BirthDate,
		Address: []requests.Address{
			{
				Use:  "home",
				Line: strings.Split(request.Address, ", "),
			},
		},
		Extension: []requests.Extension{
			{
				Url:         "http://example.org/fhir/StructureDefinition/education",
				ValueString: request.Education,
			},
		},
	}
}
