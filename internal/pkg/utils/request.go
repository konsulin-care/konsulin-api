package utils

import (
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/requests"
	"net/http"
	"strconv"
	"strings"
)

func BuildPaginationRequest(r *http.Request) *requests.Pagination {
	pageStr := r.URL.Query().Get("page")
	pageSizeStr := r.URL.Query().Get("page_size")

	page, err := strconv.Atoi(pageStr)
	if err != nil || page <= 0 {
		page = 1
	}

	pageSize, err := strconv.Atoi(pageSizeStr)
	if err != nil || pageSize <= 0 {
		pageSize = 10
	}

	return &requests.Pagination{
		Page:     page,
		PageSize: pageSize,
	}
}

func BuildFhirPatientRegistrationRequest(username, email string) *requests.PatientFhir {
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

func BuildFhirPatientUpdateProfileRequest(request *requests.UpdateProfile, patientID string) *requests.PatientFhir {
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

func BuildFhirPractitionerRegistrationRequest(username, email string) *requests.PractitionerFhir {
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

func BuildFhirPractitionerUpdateProfileRequest(request *requests.UpdateProfile, practitionerID string) *requests.PractitionerFhir {
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

func BuildFhirPractitionerDeactivateRequest(practitionerID string) *requests.PractitionerFhir {
	return &requests.PractitionerFhir{
		ResourceType: constvars.ResourcePractitioner,
		ID:           practitionerID,
		Active:       false,
	}
}

func BuildFhirPatientDeactivateRequest(patientID string) *requests.PatientFhir {
	return &requests.PatientFhir{
		ResourceType: constvars.ResourcePatient,
		ID:           patientID,
		Active:       false,
	}
}

func BuildPractitionerRolesBundleRequestByPractitionerID(practitionerID string, organizationIDs []string) interface{} {
	practitionerRoles := make([]requests.PractitionerRole, len(organizationIDs))

	for i, orgID := range organizationIDs {
		practitionerReference := requests.Reference{
			Reference: "Practitioner/" + practitionerID,
		}
		organizationReference := requests.Reference{
			Reference: "Organization/" + orgID,
		}

		practitionerRoles[i] = requests.PractitionerRole{
			ResourceType: "PractitionerRole",
			Practitioner: practitionerReference,
			Organization: organizationReference,
		}
	}

	bundle := map[string]interface{}{
		"resourceType": "Bundle",
		"type":         "transaction",
		"entry":        []interface{}{},
	}

	for _, practitionerRole := range practitionerRoles {
		entry := map[string]interface{}{
			"resource": practitionerRole,
			"request": map[string]string{
				"method": "POST",
				"url":    "PractitionerRole",
			},
		}
		bundle["entry"] = append(bundle["entry"].([]interface{}), entry)
	}

	return bundle
}

func BuildFhirPatientReactivateRequest(patientID string) *requests.PatientFhir {
	return &requests.PatientFhir{
		ResourceType: constvars.ResourcePatient,
		ID:           patientID,
		Active:       true,
	}
}

func BuildFhirPractitionerReactivateRequest(practitionerID string) *requests.PractitionerFhir {
	return &requests.PractitionerFhir{
		ResourceType: constvars.ResourcePractitioner,
		ID:           practitionerID,
		Active:       true,
	}
}

func ConvertToModelAvailableTimes(availableTimes []requests.AvailableTimeRequest) []requests.AvailableTime {
	var result []requests.AvailableTime
	for _, at := range availableTimes {
		result = append(result, requests.AvailableTime{
			DaysOfWeek:         at.DaysOfWeek,
			AvailableStartTime: at.AvailableStartTime,
			AvailableEndTime:   at.AvailableEndTime,
		})
	}
	return result
}
