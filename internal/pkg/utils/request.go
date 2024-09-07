package utils

import (
	"fmt"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/dto/responses"
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

func BuildFhirPatientRegistrationRequest(username, email string) *requests.Patient {
	return &requests.Patient{
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

func BuildFhirPatientUpdateProfileRequest(request *requests.UpdateProfile, patientID string) *requests.Patient {
	var extensions []requests.Extension
	for _, education := range request.Educations {
		extensions = append(extensions, requests.Extension{
			Url:         "http://example.org/fhir/StructureDefinition/education",
			ValueString: education,
		})
	}

	return &requests.Patient{
		ResourceType: constvars.ResourcePatient,
		ID:           patientID,
		Active:       true,
		Name: []requests.HumanName{
			{
				Use:   "official",
				Given: []string{request.Fullname},
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

func BuildFhirPractitionerRegistrationRequest(username, email string) *requests.Practitioner {
	return &requests.Practitioner{
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

func BuildFhirPractitionerUpdateProfileRequest(request *requests.UpdateProfile, practitionerID string) *requests.Practitioner {
	var extensions []requests.Extension
	for _, education := range request.Educations {
		extensions = append(extensions, requests.Extension{
			Url:         "http://example.org/fhir/StructureDefinition/education",
			ValueString: education,
		})
	}

	return &requests.Practitioner{
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

func BuildFhirPractitionerDeactivateRequest(practitionerID string) *requests.Practitioner {
	return &requests.Practitioner{
		ResourceType: constvars.ResourcePractitioner,
		ID:           practitionerID,
		Active:       false,
	}
}

func BuildFhirPatientDeactivateRequest(patientID string) *requests.Patient {
	return &requests.Patient{
		ResourceType: constvars.ResourcePatient,
		ID:           patientID,
		Active:       false,
	}
}

func BuildPractitionerRolesBundleRequestByPractitionerID(practitionerID string, organizationIDs []string) interface{} {
	practitionerRoles := make([]requests.PractitionerRole, len(organizationIDs))

	for i, orgID := range organizationIDs {
		practitionerReference := requests.Reference{
			Reference: fmt.Sprintf("%s/%s", constvars.ResourcePractitioner, practitionerID),
		}
		organizationReference := requests.Reference{
			Reference: fmt.Sprintf("%s/%s", constvars.ResourceOrganization, orgID),
		}

		practitionerRoles[i] = requests.PractitionerRole{
			ResourceType: constvars.ResourcePractitionerRole,
			Practitioner: practitionerReference,
			Organization: organizationReference,
			Active:       true,
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

func BuildFhirPatientReactivateRequest(patientID string) *requests.Patient {
	return &requests.Patient{
		ResourceType: constvars.ResourcePatient,
		ID:           patientID,
		Active:       true,
	}
}

func BuildFhirPractitionerReactivateRequest(practitionerID string) *requests.Practitioner {
	return &requests.Practitioner{
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

func ConvertToAvailableTimesResponse(availableTimes []responses.AvailableTime) []responses.AvailableTimeResponse {
	result := make([]responses.AvailableTimeResponse, 0, len(availableTimes))
	for _, at := range availableTimes {
		result = append(result, responses.AvailableTimeResponse{
			DaysOfWeek:         at.DaysOfWeek,
			AvailableStartTime: at.AvailableStartTime,
			AvailableEndTime:   at.AvailableEndTime,
		})
	}
	return result
}
