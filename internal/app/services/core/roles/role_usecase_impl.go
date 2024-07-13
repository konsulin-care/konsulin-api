package roles

import (
	"konsulin-service/internal/app/services/fhir_spark/patients"
	"konsulin-service/internal/app/services/fhir_spark/practitioners"
)

type roleUsecase struct {
	RoleRepository         RoleRepository
	PatientFhirClient      patients.PatientFhirClient
	PractitionerFhirClient practitioners.PractitionerFhirClient
}

func NewRoleUsecase(
	roleMongoRepository RoleRepository,
	patientFhirClient patients.PatientFhirClient,
	practitionerFhirClient practitioners.PractitionerFhirClient,
) RoleUsecase {
	return &roleUsecase{
		RoleRepository:         roleMongoRepository,
		PatientFhirClient:      patientFhirClient,
		PractitionerFhirClient: practitionerFhirClient,
	}
}
