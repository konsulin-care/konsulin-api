package constvars

// FHIR JSON field name constants used when building raw maps for bundle entries,
// transaction payloads, and other FHIR resource representations.
const (
	FhirFieldResourceType = "resourceType"
	FhirFieldReference    = "reference"
	FhirFieldRequest      = "request"
	FhirFieldResource     = "resource"
	FhirFieldMethod       = "method"
	FhirFieldURL          = "url"
	FhirFieldEntry        = "entry"
	FhirFieldStatus       = "status"
	FhirFieldStart        = "start"
	FhirFieldEnd          = "end"
	FhirFieldMeta         = "meta"
	FhirFieldCode         = "code"
	FhirFieldTag          = "tag"
)

// FHIR bundle type constants.
const (
	FhirBundleTypeTransaction = "transaction"
)

// FHIR bundle field constants.
const (
	FhirBundleFieldType = "type"
)

// FHIR gjson path constants used when extracting fields from FHIR JSON bodies.
const (
	FhirGJSONPathSubjectRef = "subject.reference"
)

// FHIR address and telecom use constants.
const (
	FhirAddressUseHome   = "home"
	FhirAddressUseWork   = "work"
	FhirTelecomUseMobile = "mobile"
)

// FHIR extension URL constants.
const (
	FhirEducationExtensionURL = "http://example.org/fhir/StructureDefinition/education"
)

// FHIR resource type constants not already defined in fhir.go.
const (
	ResourceBundle = "Bundle"
)

// Healthcare service unit constants.
const (
	HealthcareServiceUnitMinutes = "minutes"
	HealthcareServiceCodeMin     = "min"
)

// Environment name constants.
const (
	EnvLocal       = "local"
	EnvDev         = "dev"
	EnvDevelopment = "development"
	EnvTest        = "test"
	EnvProduction  = "production"
)
