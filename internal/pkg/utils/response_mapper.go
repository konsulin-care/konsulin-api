package utils

import (
	"konsulin-service/internal/pkg/dto/responses"
	"konsulin-service/internal/pkg/fhir_dto"
	"strings"
)

func ConvertOrganizationToClinicDetailResponse(org fhir_dto.Organization) responses.Clinic {
	clinic := responses.Clinic{
		ID:          org.ID,
		ClinicName:  org.Name,
		Affiliation: org.PartOf.Display,
	}

	if len(org.Address) > 0 {
		clinic.Address = strings.Join(org.Address[0].Line, ", ")
		if org.Address[0].City != "" {
			clinic.Address += ", " + org.Address[0].City
		}
		if org.Address[0].State != "" {
			clinic.Address += ", " + org.Address[0].State
		}
		if org.Address[0].PostalCode != "" {
			clinic.Address += " " + org.Address[0].PostalCode
		}
		if org.Address[0].Country != "" {
			clinic.Address += ", " + org.Address[0].Country
		}
	}

	for _, codeableConcept := range org.Type {
		for _, coding := range codeableConcept.Coding {
			clinic.Tags = append(clinic.Tags, coding.Display)
		}
	}

	return clinic
}

func ConvertOrganizationToClinicResponse(org fhir_dto.Organization) responses.Clinic {
	clinic := responses.Clinic{
		ID:         org.ID,
		ClinicName: org.Name,
	}

	for _, codeableConcept := range org.Type {
		for _, coding := range codeableConcept.Coding {
			clinic.Tags = append(clinic.Tags, coding.Display)
		}
	}
	return clinic
}
