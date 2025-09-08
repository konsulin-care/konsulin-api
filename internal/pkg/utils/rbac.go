package utils

import (
	"net/url"
	"strings"
)

func PathMatch(requestPath, policyPath string) bool {
	requestURL, err := url.Parse(requestPath)
	if err != nil {
		return false
	}

	policyURL, err := url.Parse(policyPath)
	if err != nil {
		return false
	}

	if requestURL.Path != policyURL.Path {
		return false
	}

	if len(policyURL.RawQuery) == 0 {
		return true
	}

	return requestURL.RawQuery == policyURL.RawQuery
}

func NormalizePath(rawURL string) string {
	path := strings.TrimPrefix(rawURL, "/")

	if !strings.HasPrefix(path, "fhir/") {
		path = "fhir/" + path
	}

	return "/" + path
}

func RequiresPatientOwnership(resourceType string) bool {
	patientSpecificResources := map[string]bool{
		"Patient":                  true,
		"Appointment":              true,
		"Observation":              true,
		"QuestionnaireResponse":    true,
		"Encounter":                true,
		"Condition":                true,
		"AllergyIntolerance":       true,
		"MedicationRequest":        true,
		"Procedure":                true,
		"DiagnosticReport":         true,
		"ImagingStudy":             true,
		"DocumentReference":        true,
		"CarePlan":                 true,
		"Goal":                     true,
		"RiskAssessment":           true,
		"FamilyMemberHistory":      true,
		"Immunization":             true,
		"MedicationAdministration": true,
		"MedicationDispense":       true,
		"MedicationStatement":      true,
		"Coverage":                 true,
		"Claim":                    true,
		"ExplanationOfBenefit":     true,
		"PaymentNotice":            true,
		"PaymentReconciliation":    true,
		"Account":                  true,
		"ChargeItem":               true,
		"Invoice":                  true,
	}

	return patientSpecificResources[resourceType]
}

func RequiresPractitionerOwnership(resourceType string) bool {
	practitionerSpecificResources := map[string]bool{
		"Practitioner":                true,
		"Schedule":                    true,
		"Encounter":                   true,
		"Observation":                 true,
		"DiagnosticReport":            true,
		"Procedure":                   true,
		"MedicationRequest":           true,
		"CarePlan":                    true,
		"QuestionnaireResponse":       true,
		"DocumentReference":           true,
		"Communication":               true,
		"CommunicationRequest":        true,
		"Task":                        true,
		"Consent":                     true,
		"Contract":                    true,
		"CoverageEligibilityRequest":  true,
		"CoverageEligibilityResponse": true,
		"Claim":                       true,
		"ClaimResponse":               true,
		"ExplanationOfBenefit":        true,
		"PaymentNotice":               true,
		"PaymentReconciliation":       true,
		"Account":                     true,
		"ChargeItem":                  true,
		"Invoice":                     true,
	}

	return practitionerSpecificResources[resourceType]
}

func IsPublicResource(resourceType string) bool {
	publicResources := map[string]bool{
		"Questionnaire":           true,
		"QuestionnaireResponse":   true,
		"ResearchStudy":           true,
		"Organization":            true,
		"Location":                true,
		"HealthcareService":       true,
		"PractitionerRole":        true,
		"Slot":                    true,
		"CodeSystem":              true,
		"ValueSet":                true,
		"ConceptMap":              true,
		"StructureDefinition":     true,
		"OperationDefinition":     true,
		"SearchParameter":         true,
		"CompartmentDefinition":   true,
		"GraphDefinition":         true,
		"ImplementationGuide":     true,
		"CapabilityStatement":     true,
		"MessageDefinition":       true,
		"ActivityDefinition":      true,
		"PlanDefinition":          true,
		"Library":                 true,
		"Measure":                 true,
		"MeasureReport":           true,
		"TestScript":              true,
		"TestReport":              true,
		"Subscription":            true,
		"SubscriptionTopic":       true,
		"VerificationResult":      true,
		"Requirements":            true,
		"ExampleScenario":         true,
		"SpecimenDefinition":      true,
		"NamingSystem":            true,
		"TerminologyCapabilities": true,
	}

	return publicResources[resourceType]
}

func ExtractResourceTypeFromPath(path string) string {
	u, err := url.Parse(path)
	if err != nil {

		parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
		if len(parts) >= 2 && strings.EqualFold(parts[0], "fhir") {
			return parts[1]
		} else if len(parts) >= 1 {
			return parts[0]
		}
		return ""
	}

	parts := strings.Split(strings.TrimPrefix(u.Path, "/"), "/")

	if len(parts) >= 2 && strings.EqualFold(parts[0], "fhir") {
		return parts[1]
	} else if len(parts) >= 1 {
		return parts[0]
	}

	return ""
}
