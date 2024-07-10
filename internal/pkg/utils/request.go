package utils

import (
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/requests"
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
		Extension: []requests.Extension{
			{
				Url:         "http://example.org/fhir/StructureDefinition/education",
				ValueString: request.Education,
			},
		},
	}
}
