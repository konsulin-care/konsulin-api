package utils

import (
	"encoding/json"
	"fmt"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/dto/responses"
	"konsulin-service/internal/pkg/fhir_dto"
	"net/http"
	"strconv"
	"strings"
)

func ParseJSONBody(body []byte) (map[string]interface{}, error) {
	var data map[string]interface{}
	err := json.Unmarshal(body, &data)
	if err != nil {
		return nil, err
	}
	return data, nil
}

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

func BuildQueryParamsRequest(r *http.Request) *requests.QueryParams {
	searchStr := r.URL.Query().Get(constvars.URLQueryParamSearch)
	fetchType := r.URL.Query().Get(constvars.URLQueryParamFetchType)

	if fetchType != constvars.QueryParamFetchTypeUpcoming {
		fetchType = ""
	}

	return &requests.QueryParams{
		Search:    searchStr,
		FetchType: fetchType,
	}
}

func BuildFhirPatientRegistrationRequest(username, email string) *fhir_dto.Patient {
	return &fhir_dto.Patient{
		ResourceType: constvars.ResourcePatient,
		Telecom: []fhir_dto.ContactPoint{
			{
				System: fhir_dto.ContactPointSystemEmail,
				Value:  email,
				Use:    constvars.FhirAddressUseHome,
			},
		},
	}
}

func BuildFhirPatientWhatsAppRegistrationRequest(phoneNumber string) *fhir_dto.Patient {
	return &fhir_dto.Patient{
		ResourceType: constvars.ResourcePatient,
		Telecom: []fhir_dto.ContactPoint{
			{
				System: fhir_dto.ContactPointSystemPhone,
				Value:  phoneNumber,
				Use:    constvars.FhirTelecomUseMobile,
			},
		},
	}
}

func BuildFhirPatientUpdateProfileRequest(request *requests.UpdateProfile, patientID string) *fhir_dto.Patient {
	var extensions []fhir_dto.Extension
	for _, education := range request.Educations {
		extensions = append(extensions, fhir_dto.Extension{
			Url:         constvars.FhirEducationExtensionURL,
			ValueString: education,
		})
	}

	return &fhir_dto.Patient{
		ResourceType: constvars.ResourcePatient,
		ID:           patientID,
		Active:       true,
		Name: []fhir_dto.HumanName{
			{
				Use:   "official",
				Given: []string{request.Fullname},
			},
		},
		Telecom: []fhir_dto.ContactPoint{
			{
				System: fhir_dto.ContactPointSystemEmail,
				Value:  request.Email,
				Use:    constvars.FhirAddressUseHome,
			},
			{
				System: fhir_dto.ContactPointSystemPhone,
				Value:  request.WhatsAppNumber,
				Use:    constvars.FhirTelecomUseMobile,
			},
		},
		Gender:    request.Gender,
		BirthDate: request.BirthDate,
		Address: []fhir_dto.Address{
			{
				Use:  constvars.FhirAddressUseHome,
				Line: strings.Split(request.Address, ", "),
			},
		},
		Extension: extensions,
	}
}

func BuildFhirPractitionerRegistrationRequest(username, email string) *fhir_dto.Practitioner {
	return &fhir_dto.Practitioner{
		ResourceType: constvars.ResourcePractitioner,
		Telecom: []fhir_dto.ContactPoint{
			{
				System: fhir_dto.ContactPointSystemEmail,
				Value:  email,
				Use:    constvars.FhirAddressUseWork,
			},
		},
	}
}

func BuildFhirPractitionerWhatsAppRegistrationRequest(phoneNumber string) *fhir_dto.Practitioner {
	return &fhir_dto.Practitioner{
		ResourceType: constvars.ResourcePractitioner,
		Telecom: []fhir_dto.ContactPoint{
			{
				System: fhir_dto.ContactPointSystemPhone,
				Value:  phoneNumber,
				Use:    "mobile",
			},
		},
	}
}

func BuildFhirPractitionerUpdateProfileRequest(request *requests.UpdateProfile, practitionerID string) *fhir_dto.Practitioner {
	var extensions []fhir_dto.Extension
	for _, education := range request.Educations {
		extensions = append(extensions, fhir_dto.Extension{
			Url:         constvars.FhirEducationExtensionURL,
			ValueString: education,
		})
	}

	return &fhir_dto.Practitioner{
		ResourceType: constvars.ResourcePractitioner,
		ID:           practitionerID,
		Active:       true,
		Name: []fhir_dto.HumanName{
			{
				Use:    "official",
				Family: request.Fullname,
			},
		},
		Telecom: []fhir_dto.ContactPoint{
			{
				System: fhir_dto.ContactPointSystemEmail,
				Value:  request.Email,
				Use:    constvars.FhirAddressUseWork,
			},
			{
				System: fhir_dto.ContactPointSystemPhone,
				Value:  request.WhatsAppNumber,
				Use:    constvars.FhirTelecomUseMobile,
			},
		},
		Gender:    request.Gender,
		BirthDate: request.BirthDate,
		Address: []fhir_dto.Address{
			{
				Use:  constvars.FhirAddressUseWork,
				Line: strings.Split(request.Address, ", "),
			},
		},
		Extension: extensions,
	}
}

func BuildFhirPractitionerDeactivateRequest(practitionerID string) *fhir_dto.Practitioner {
	return &fhir_dto.Practitioner{
		ResourceType: constvars.ResourcePractitioner,
		ID:           practitionerID,
		Active:       false,
	}
}

func BuildFhirPatientDeactivateRequest(patientID string) *fhir_dto.Patient {
	return &fhir_dto.Patient{
		ResourceType: constvars.ResourcePatient,
		ID:           patientID,
		Active:       false,
	}
}

func BuildPractitionerRolesBundleRequestByPractitionerID(practitionerID string, organizationIDs []string) interface{} {
	practitionerRoles := make([]fhir_dto.PractitionerRole, len(organizationIDs))

	for i, orgID := range organizationIDs {
		practitionerReference := fhir_dto.Reference{
			Reference: fmt.Sprintf("%s/%s", constvars.ResourcePractitioner, practitionerID),
		}
		organizationReference := fhir_dto.Reference{
			Reference: fmt.Sprintf("%s/%s", constvars.ResourceOrganization, orgID),
		}

		practitionerRoles[i] = fhir_dto.PractitionerRole{
			ResourceType: constvars.ResourcePractitionerRole,
			Practitioner: practitionerReference,
			Organization: organizationReference,
			Active:       true,
		}
	}

	bundle := map[string]interface{}{
		constvars.FhirFieldResourceType: constvars.ResourceBundle,
		constvars.FhirBundleFieldType:   constvars.FhirBundleTypeTransaction,
		constvars.FhirFieldEntry:        []interface{}{},
	}

	for _, practitionerRole := range practitionerRoles {
		entry := map[string]interface{}{
			constvars.FhirFieldResource: practitionerRole,
			constvars.FhirFieldRequest: map[string]string{
				constvars.FhirFieldMethod: constvars.MethodPost,
				constvars.FhirFieldURL:    "PractitionerRole",
			},
		}
		bundle[constvars.FhirFieldEntry] = append(bundle[constvars.FhirFieldEntry].([]interface{}), entry)
	}

	return bundle
}

func BuildFhirPatientReactivateRequest(patientID string) *fhir_dto.Patient {
	return &fhir_dto.Patient{
		ResourceType: constvars.ResourcePatient,
		ID:           patientID,
		Active:       true,
	}
}

func BuildFhirPractitionerReactivateRequest(practitionerID string) *fhir_dto.Practitioner {
	return &fhir_dto.Practitioner{
		ResourceType: constvars.ResourcePractitioner,
		ID:           practitionerID,
		Active:       true,
	}
}

func ConvertToModelAvailableTimes(availableTimes []requests.AvailableTimeRequest) []fhir_dto.AvailableTime {
	var result []fhir_dto.AvailableTime
	for _, at := range availableTimes {
		result = append(result, fhir_dto.AvailableTime{
			DaysOfWeek:         at.DaysOfWeek,
			AvailableStartTime: at.AvailableStartTime,
			AvailableEndTime:   at.AvailableEndTime,
		})
	}
	return result
}

func ConvertToAvailableTimesResponse(availableTimes []fhir_dto.AvailableTime) []responses.AvailableTimeResponse {
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
