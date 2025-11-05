package fhir_dto

type Condition struct {
	ResourceType        string              `json:"resourceType"`
	ID                  string              `json:"id,omitempty"`
	Meta                Meta                `json:"meta,omitempty"`
	Identifier          []Identifier        `json:"identifier,omitempty"`
	ClinicalStatus      *CodeableConcept    `json:"clinicalStatus,omitempty"`
	VerificationStatus  *CodeableConcept    `json:"verificationStatus,omitempty"`
	Category            []CodeableConcept   `json:"category,omitempty"`
	Severity            *CodeableConcept    `json:"severity,omitempty"`
	Code                *CodeableConcept    `json:"code,omitempty"`
	BodySite            []CodeableConcept   `json:"bodySite,omitempty"`
	Subject             Reference           `json:"subject"`
	Encounter           *Reference          `json:"encounter,omitempty"`
	OnsetDateTime       string              `json:"onsetDateTime,omitempty"`
	OnsetPeriod         *Period             `json:"onsetPeriod,omitempty"`
	AbatementDateTime   string              `json:"abatementDateTime,omitempty"`
	RecordedDate        string              `json:"recordedDate,omitempty"`
	Recorder            *Reference          `json:"recorder,omitempty"`
	Asserter            *Reference          `json:"asserter,omitempty"`
	Stage               []ConditionStage    `json:"stage,omitempty"`
	Evidence            []ConditionEvidence `json:"evidence,omitempty"`
	Note                []Annotation        `json:"note,omitempty"`
}

type ConditionStage struct {
	Summary    *CodeableConcept `json:"summary,omitempty"`
	Assessment []Reference      `json:"assessment,omitempty"`
	Type       *CodeableConcept `json:"type,omitempty"`
}

type ConditionEvidence struct {
	Code   []CodeableConcept `json:"code,omitempty"`
	Detail []Reference       `json:"detail,omitempty"`
}

