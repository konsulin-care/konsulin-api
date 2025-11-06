package fhir_dto

import (
	"net/url"
	"time"
)

// CreateServiceRequestInput is the payload sent to FHIR when creating a ServiceRequest.
type CreateServiceRequestInput struct {
	ResourceType       string     `json:"resourceType"`
	Status             string     `json:"status,omitempty"`
	Intent             string     `json:"intent,omitempty"`
	Subject            Reference  `json:"subject,omitempty"`
	Requester          *Reference `json:"requester,omitempty"`
	OccurrenceDateTime string     `json:"occurrenceDateTime,omitempty"`
	AuthoredOn         time.Time  `json:"authoredOn,omitempty"`
	// instantiatesUri corresponds to FHIR ServiceRequest.instantiatesUri (0..*)
	// See: https://hl7.org/fhir/R4/servicerequest.html#resource
	InstantiatesUri []string     `json:"instantiatesUri,omitempty"`
	Note            []Annotation `json:"note,omitempty"`
}

// CreateServiceRequestOutput represents the ServiceRequest returned by FHIR.
type CreateServiceRequestOutput struct {
	ResourceType       string    `json:"resourceType"`
	ID                 string    `json:"id,omitempty"`
	Meta               Meta      `json:"meta,omitempty"`
	Status             string    `json:"status,omitempty"`
	Intent             string    `json:"intent,omitempty"`
	Subject            Reference `json:"subject,omitempty"`
	Requester          Reference `json:"requester,omitempty"`
	OccurrenceDateTime string    `json:"occurrenceDateTime,omitempty"`
	AuthoredOn         time.Time `json:"authoredOn,omitempty"`
	// instantiatesUri corresponds to FHIR ServiceRequest.instantiatesUri (0..*)
	InstantiatesUri []string     `json:"instantiatesUri,omitempty"`
	Note            []Annotation `json:"note,omitempty"`
}

// GetServiceRequestOutput represents the response when fetching a specific version
// of a ServiceRequest resource from FHIR.
type GetServiceRequestOutput struct {
	ResourceType       string    `json:"resourceType"`
	ID                 string    `json:"id,omitempty"`
	Meta               Meta      `json:"meta,omitempty"`
	Status             string    `json:"status,omitempty"`
	Intent             string    `json:"intent,omitempty"`
	Subject            Reference `json:"subject,omitempty"`
	Requester          Reference `json:"requester,omitempty"`
	OccurrenceDateTime string    `json:"occurrenceDateTime,omitempty"`
	AuthoredOn         time.Time `json:"authoredOn,omitempty"`
	// instantiatesUri corresponds to FHIR ServiceRequest.instantiatesUri (0..*)
	InstantiatesUri []string     `json:"instantiatesUri,omitempty"`
	Note            []Annotation `json:"note,omitempty"`
}

// SearchServiceRequestInput contains search parameters for querying ServiceRequest resources.
type SearchServiceRequestInput struct {
	ID string
}

// ToQueryString converts search input to URL query parameters.
func (s *SearchServiceRequestInput) ToQueryString() url.Values {
	params := url.Values{}
	if s.ID != "" {
		params.Set("_id", s.ID)
	}
	return params
}

// UpdateServiceRequestInput represents the full ServiceRequest resource for PUT updates.
// Based on FHIR R4 ServiceRequest: https://hl7.org/fhir/R4/servicerequest.html
type UpdateServiceRequestInput struct {
	ResourceType            string            `json:"resourceType"`
	ID                      string            `json:"id,omitempty"`
	Meta                    Meta              `json:"meta,omitempty"`
	ImplicitRules           string            `json:"implicitRules,omitempty"`
	Language                string            `json:"language,omitempty"`
	Text                    *Narrative        `json:"text,omitempty"`
	Contained               []interface{}     `json:"contained,omitempty"`
	Extension               []Extension       `json:"extension,omitempty"`
	ModifierExtension       []Extension       `json:"modifierExtension,omitempty"`
	Identifier              []Identifier      `json:"identifier,omitempty"`
	InstantiatesCanonical   []string          `json:"instantiatesCanonical,omitempty"`
	InstantiatesUri         []string          `json:"instantiatesUri,omitempty"`
	BasedOn                 []Reference       `json:"basedOn,omitempty"`
	Replaces                []Reference       `json:"replaces,omitempty"`
	Requisition             *Identifier       `json:"requisition,omitempty"`
	Status                  string            `json:"status"`
	Intent                  string            `json:"intent"`
	Category                []CodeableConcept `json:"category,omitempty"`
	Priority                string            `json:"priority,omitempty"`
	DoNotPerform            bool              `json:"doNotPerform,omitempty"`
	Code                    *CodeableConcept  `json:"code,omitempty"`
	OrderDetail             []CodeableConcept `json:"orderDetail,omitempty"`
	QuantityQuantity        *Quantity         `json:"quantityQuantity,omitempty"`
	QuantityRatio           interface{}       `json:"quantityRatio,omitempty"`
	QuantityRange           *Range            `json:"quantityRange,omitempty"`
	Subject                 Reference         `json:"subject"`
	Encounter               *Reference        `json:"encounter,omitempty"`
	OccurrenceDateTime      string            `json:"occurrenceDateTime,omitempty"`
	OccurrencePeriod        *Period           `json:"occurrencePeriod,omitempty"`
	OccurrenceTiming        *Timing           `json:"occurrenceTiming,omitempty"`
	AsNeededBoolean         bool              `json:"asNeededBoolean,omitempty"`
	AsNeededCodeableConcept *CodeableConcept  `json:"asNeededCodeableConcept,omitempty"`
	AuthoredOn              time.Time         `json:"authoredOn,omitempty"`
	Requester               *Reference        `json:"requester,omitempty"`
	PerformerType           *CodeableConcept  `json:"performerType,omitempty"`
	Performer               []Reference       `json:"performer,omitempty"`
	LocationCode            []CodeableConcept `json:"locationCode,omitempty"`
	LocationReference       []Reference       `json:"locationReference,omitempty"`
	ReasonCode              []CodeableConcept `json:"reasonCode,omitempty"`
	ReasonReference         []Reference       `json:"reasonReference,omitempty"`
	Insurance               []Reference       `json:"insurance,omitempty"`
	SupportingInfo          []Reference       `json:"supportingInfo,omitempty"`
	Specimen                []Reference       `json:"specimen,omitempty"`
	BodySite                []CodeableConcept `json:"bodySite,omitempty"`
	Note                    []Annotation      `json:"note,omitempty"`
	PatientInstruction      string            `json:"patientInstruction,omitempty"`
	RelevantHistory         []Reference       `json:"relevantHistory,omitempty"`
}

// ServiceRequestBundle represents a FHIR Bundle containing ServiceRequest search results.
type ServiceRequestBundle struct {
	ResourceType string                      `json:"resourceType"`
	Type         string                      `json:"type"`
	Total        int                         `json:"total"`
	Entry        []ServiceRequestBundleEntry `json:"entry,omitempty"`
}

// ServiceRequestBundleEntry represents a single entry in a ServiceRequest search bundle.
type ServiceRequestBundleEntry struct {
	FullUrl  string                  `json:"fullUrl,omitempty"`
	Resource GetServiceRequestOutput `json:"resource"`
}
