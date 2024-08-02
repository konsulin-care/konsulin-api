package utils

import (
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/requests"
	"net/http"
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

func BuildFhirPatientUpdateRequest(request *requests.UpdateProfile, patientID string) *requests.PatientFhir {
	var extensions []requests.Extension
	for _, education := range request.Educations {
		extensions = append(extensions, requests.Extension{
			Url:         "http://example.org/fhir/StructureDefinition/education",
			ValueString: education,
		})
	}

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
		Extension: extensions,
	}
}

func BuildFhirPractitionerRequest(username, email string) *requests.PractitionerFhir {
	return &requests.PractitionerFhir{
		ResourceType: constvars.ResourcePractitioner,
		Telecom: []requests.ContactPoint{
			{
				System: "email",
				Value:  email,
				Use:    "work",
			},
		},
	}
}

func BuildFhirPractitionerUpdateRequest(request *requests.UpdateProfile, practitionerID string) *requests.PractitionerFhir {
	var extensions []requests.Extension
	for _, education := range request.Educations {
		extensions = append(extensions, requests.Extension{
			Url:         "http://example.org/fhir/StructureDefinition/education",
			ValueString: education,
		})
	}

	return &requests.PractitionerFhir{
		ResourceType: constvars.ResourcePractitioner,
		ID:           practitionerID,
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
				Use:    "work",
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
				Use:  "work",
				Line: strings.Split(request.Address, ", "),
			},
		},
		Extension: extensions,
	}
}

func BuildUpdateUserProfileRequest(r *http.Request) (*requests.UpdateProfile, error) {
	request := new(requests.UpdateProfile)

	request.Fullname = r.FormValue("fullname")
	request.Email = r.FormValue("email")
	request.BirthDate = r.FormValue("birth_date")
	request.WhatsAppNumber = r.FormValue("whatsapp_number")
	request.Address = r.FormValue("address")
	request.Gender = r.FormValue("gender")
	request.Educations = r.Form["educations"]

	file, fileHeader, err := r.FormFile("profile_picture")
	if err == nil {
		defer file.Close()
		request.ProfilePicture = make([]byte, fileHeader.Size)
		_, err := file.Read(request.ProfilePicture)
		if err != nil {
			return nil, err
		}
		request.ProfilePictureName = fileHeader.Filename
	}

	return request, nil
}
